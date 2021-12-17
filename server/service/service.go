package service

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/golang/glog"

	"github.com/karagog/clock-go"
	"github.com/karagog/db-provider/server/lessor"
	pb "github.com/karagog/db-provider/server/proto"
)

type Service struct {
	pb.UnimplementedIntegrationTestServer

	clock    clock.Clock
	initDone chan bool
	lessor   *lessor.Lessor
}

func New(clock clock.Clock) *Service {
	return &Service{
		clock:    clock,
		initDone: make(chan bool),
	}
}

func (s *Service) GetStatus(ctx context.Context, _ *pb.GetStatusRequest) (*pb.GetStatusResponse, error) {
	return &pb.GetStatusResponse{State: pb.GetStatusResponse_UP}, nil
}

// Sets the lessor sometime after creation, which allows the server to start providing databases.
// Until that point, the status RPC will show that we're still starting up, and
// requests for database instances will block indefinitely until it's available.
// This can only be set once, setting it twice is a fatal error.
func (s *Service) SetLessor(l *lessor.Lessor) {
	if s.lessor != nil {
		panic("lessor has already been set")
	}
	s.lessor = l
	close(s.initDone)
}

func (s *Service) GetDatabaseInstance(srv pb.IntegrationTest_GetDatabaseInstanceServer) error {
	glog.V(3).Infof("Handling GetDatabaseInstance request...")

	// Get the first message from the stream, which initiates the request.
	if _, err := srv.Recv(); err != nil {
		glog.Errorf("Error receiving first message in stream: %v", err)
		return err
	}

	// Wait here indefinitely until the provider is ready.
	for s.lessor == nil {
		select {
		case <-srv.Context().Done():
			return fmt.Errorf("client cancelled")
		case <-s.initDone:
		}
	}

	// Spawn a goroutine for consuming further messages (if any) from the client.
	// This is how we know when the client disconnects gracefully.
	clientErrCh := make(chan error, 1) // becomes readable when the client is done for any reason
	go func() {
		for {
			req, err := srv.Recv()
			if err != nil {
				clientErrCh <- err
				return
			}
			// Can safely ignore the spurious message, but it may indicate a client-side bug.
			glog.Warningf("Received unexpected message from the client: %v", req)
		}
	}()

	// Spawn another goroutine to ask the manager for an instance (as this can block indefinitely).
	var lease lessor.Lease
	var leaseErr error
	leaseCh := make(chan bool)
	ctx, cancel := context.WithCancel(srv.Context())
	go func(ctx context.Context) {
		defer func() { leaseCh <- true }()
		gotHandle, err := s.lessor.Lease(ctx)
		if err != nil {
			leaseErr = err
			return
		}
		lease = gotHandle
	}(ctx)

	leaseGranted := false

	// Cancels the goroutine that's requesting the lease and joins it so we can access its return values.
	cancelAndJoinLeaseRequest := func() {
		cancel()
		if !leaseGranted {
			<-leaseCh // join the lease thread to avoid racing when we check `lease`
		}
	}

	// Sends a response to the client. Handles errors by resetting the instance.
	sendResp := func(resp *pb.GetDatabaseInstanceResponse) error {
		if err := srv.Send(resp); err != nil {
			// Client disconnected?
			cancelAndJoinLeaseRequest()
			if lease != nil {
				s.lessor.Return(lease)
			}
			return err
		}
		return nil
	}
	if err := sendResp(&pb.GetDatabaseInstanceResponse{Status: "requesting lease"}); err != nil {
		return err
	}
	status := "waiting for lease"
	period := 10 * time.Second
	tmr := s.clock.NewTimer(period)
	for {
		select {
		case <-tmr.C():
			// Send periodic status messages.
			if err := sendResp(&pb.GetDatabaseInstanceResponse{Status: status}); err != nil {
				return err
			}
			tmr.Reset(period)
		case <-leaseCh:
			if leaseErr != nil {
				return leaseErr
			}
			leaseGranted = true
			status = "lease active"
			// Notify the client that the lease is active.
			resp := &pb.GetDatabaseInstanceResponse{
				ConnectionInfo: s.lessor.ConnectionInfo(lease),
			}
			if err := sendResp(resp); err != nil {
				return err
			}
		case err := <-clientErrCh:
			// Client is done with the lease (either they said they're done or they crashed).
			glog.V(3).Infof("Client is done: %v", err)
			cancelAndJoinLeaseRequest()
			if err == io.EOF {
				err = nil // not an error, just end of stream
			} else {
				glog.V(2).Infof("Recieved client error: %v", err)
			}
			if lease != nil {
				s.lessor.Return(lease)
			}
			return err
		case <-srv.Context().Done():
			glog.V(3).Infof("Client's request context is done")
			cancelAndJoinLeaseRequest()
			if lease != nil {
				s.lessor.Return(lease)
			}
			return nil
		}

	}
}
