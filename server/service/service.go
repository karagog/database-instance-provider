package service

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/golang/glog"

	"github.com/karagog/clock-go"
	"github.com/karagog/db-provider/server/lessor"
	pb "github.com/karagog/db-provider/server/proto"
)

type Service struct {
	pb.UnimplementedIntegrationTestServer

	Clock clock.Clock

	mu     sync.Mutex
	lessor *lessor.Lessor
}

func (s *Service) GetStatus(ctx context.Context, _ *pb.GetStatusRequest) (*pb.GetStatusResponse, error) {
	ret := &pb.GetStatusResponse{State: pb.GetStatusResponse_UNKNOWN_STATE}
	s.mu.Lock()
	if s.lessor == nil {
		ret.State = pb.GetStatusResponse_STARTING
	} else {
		ret.State = pb.GetStatusResponse_UP
	}
	s.mu.Unlock()
	return ret, nil
}

// Sets the lessor sometime after creation, which allows the server to start providing databases.
// Until that point, the status RPC will show that we're still starting up.
func (s *Service) SetLessor(l *lessor.Lessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lessor = l
}

func (s *Service) GetDatabaseInstance(srv pb.IntegrationTest_GetDatabaseInstanceServer) error {
	glog.V(3).Infof("Handling GetDatabaseInstance request...")
	var l *lessor.Lessor
	func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		l = s.lessor
	}()

	if l == nil {
		return fmt.Errorf("database provider is not ready yet - check the status RPC")
	}

	// Get the first message from the stream, which initiates the request.
	if _, err := srv.Recv(); err != nil {
		glog.Errorf("Error receiving first message in stream: %v", err)
		return err
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
		gotHandle, err := l.Lease(ctx)
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
				l.Return(lease)
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
	tmr := s.Clock.NewTimer(period)
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
				ConnectionInfo: l.ConnectionInfo(lease),
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
				l.Return(lease)
			}
			return err
		case <-srv.Context().Done():
			glog.V(3).Infof("Client's request context is done")
			cancelAndJoinLeaseRequest()
			if lease != nil {
				l.Return(lease)
			}
			return nil
		}

	}
}
