package service

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/golang/glog"
	"google.golang.org/grpc"

	"github.com/karagog/clock-go/simulated"
	"github.com/karagog/db-provider/server/lessor"
	"github.com/karagog/db-provider/server/lessor/databaseprovider/fake"
	pb "github.com/karagog/db-provider/server/proto"
)

const (
	expSilenceDur = 10 * time.Millisecond
	expMessageDur = time.Second
)

// Gives us control over the server.
type serverCtl struct {
	clock       *simulated.Clock // control the server time
	serviceAddr string           // access the server address (const)
	lessor      *lessor.Lessor   // interact with the lessor
}

// Starts up the test service listening on localhost. The service will use
// a Lessor that is backed by a single in-memory mysql instance, which makes
// it easy to test blocking scenarios simply by grabbing the lease before
// the client gets there.
//
// Call `stop` to kill the server.
func startServer(t *testing.T) (ctl *serverCtl, stop func()) {
	// Set up a local grpc instance of the test service.
	ls, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}

	s := grpc.NewServer()
	c := simulated.NewClock(time.Date(2021, 3, 26, 0, 0, 0, 0, time.UTC))
	l := lessor.New(&fake.DatabaseProvider{}, 1) // only one instance available

	// The lessor needs to run in a background context, which only ends
	// when we cancel the context.
	lessorCtx, cancelLessor := context.WithCancel(context.TODO())
	lessorDone := make(chan bool)
	go func() {
		defer close(lessorDone)
		l.Run(lessorCtx)
	}()

	pb.RegisterIntegrationTestServer(s, &Service{
		Clock:  c,
		Lessor: l,
	})
	shuttingDown := false
	done := make(chan bool)
	go func() {
		defer close(done)
		err := s.Serve(ls)
		glog.V(2).Info("Server shutting down")
		if err != nil {
			if !shuttingDown {
				glog.Fatalf("Server exited prematurely: %s", err)
			}
		}
	}()
	ctl = &serverCtl{
		clock:       c,
		serviceAddr: ls.Addr().String(),
		lessor:      l,
	}
	return ctl, func() {
		if shuttingDown {
			return // only stop it once
		}
		shuttingDown = true
		s.Stop()
		<-done

		// Now the server is stopped, so stop the lessor.
		cancelLessor()
		<-lessorDone
	}
}

// Test a nominal client-server interaction.
func TestGetDatabaseInstance(t *testing.T) {
	server, stop := startServer(t)
	defer stop()

	// There is only one lease available, so let's grab it now to cause
	// the test to block waiting for it.
	lease, err := server.lessor.Lease(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	c := newTestClient(server.serviceAddr, t)
	go c.Run()

	// The server should wait for the first request from the client.
	server.clock.Advance(time.Minute)
	c.AssertNoResponse("initial wait", t)

	// Send the first message, which initiates the lease request.
	if err := c.stream.Send(&pb.GetDatabaseInstanceRequest{}); err != nil {
		t.Fatal(err)
	}
	// Expect immediate acknowledgement of the request.
	c.GetResponse("after first message", t)

	// After some time passes waiting for the lease, the server sends a status update.
	server.clock.Advance(time.Minute)
	resp := c.GetResponse("periodic update", t)
	if resp.Status == "" {
		t.Error("Got empty status, want a status update")
	}

	// A lease becomes available. Expect an immediate update.
	server.lessor.Return(lease)
	resp = c.GetResponse("lease available", t)
	if resp.ConnectionInfo == nil {
		t.Fatal("Got nil connection info, want info")
	}

	// Check that the lease was actually taken by trying to grab it again.
	ctx, cancel := context.WithTimeout(context.TODO(), 20*time.Millisecond)
	defer cancel()
	_, err = server.lessor.Lease(ctx)
	if want := context.DeadlineExceeded; err != want {
		t.Fatalf("Got wrong error (%v), want %v", err, want)
	}

	// We hold the lease for a while, and receive more periodic updates.
	server.clock.Advance(time.Minute)
	resp = c.GetResponse("periodic update", t)
	if resp.Status == "" {
		t.Error("Got empty status, want a status update")
	}

	// If the client sends more messages, the server just ignores them.
	if err := c.stream.Send(&pb.GetDatabaseInstanceRequest{}); err != nil {
		t.Fatal(err)
	}
	c.AssertNoResponse("ignore spurious message", t)

	// When we're done with the lease, the client closes the connection.
	if err := c.stream.CloseSend(); err != nil {
		t.Fatal(err)
	}
	c.AssertError("closed connection", io.EOF, t) // normal end-of-stream sentinel error
	c.Wait(t)

	// Check that the lease was released by trying to lease it again.
	ctx, cancel = context.WithTimeout(context.TODO(), time.Second)
	defer cancel()
	if _, err = server.lessor.Lease(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestServerDisconnect(t *testing.T) {
	server, stop := startServer(t)
	defer stop()

	c := newTestClient(server.serviceAddr, t)
	go c.Run()
	if err := c.stream.Send(&pb.GetDatabaseInstanceRequest{}); err != nil {
		t.Fatal(err)
	}
	c.GetResponse("initial message", t)
	c.GetResponse("lease available", t)

	// If the server stops unexpectedly, we expect to immediately receive an
	// error on the client side. That is how the client knows to stop using the lease.
	stop()
	c.AssertError("server stopped", nil, t)
}

func TestClientGivesUpWithoutLease(t *testing.T) {
	server, stop := startServer(t)
	defer stop()

	// Grab and hold the only lease so the client can't get it.
	if _, err := server.lessor.Lease(context.TODO()); err != nil {
		t.Fatal(err)
	}

	c := newTestClient(server.serviceAddr, t)
	go c.Run()
	if err := c.stream.Send(&pb.GetDatabaseInstanceRequest{}); err != nil {
		t.Fatal(err)
	}
	c.GetResponse("after first message", t)

	// Close the connection before a lease becomes available.
	if err := c.stream.CloseSend(); err != nil {
		t.Fatal(err)
	}
	c.AssertError("closed connection", io.EOF, t)
}

func TestClientClosesConnectionBeforeFirstMessage(t *testing.T) {
	server, stop := startServer(t)
	defer stop()

	c := newTestClient(server.serviceAddr, t)
	go c.Run()

	if err := c.stream.CloseSend(); err != nil {
		t.Fatal(err)
	}
	// This is not a EOF because the server treats it as a call failure, since
	// nothing was requested and no work was done.
	c.AssertError("premature broken connection", nil, t)
}

// The test client receives responses from the server in its Run() method and
// exposes the messages it receives via the channels.
type testClient struct {
	stream pb.IntegrationTest_GetDatabaseInstanceClient
	respCh chan *pb.GetDatabaseInstanceResponse
	errCh  chan error
}

func newTestClient(addr string, t *testing.T) *testClient {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	stream, err := pb.NewIntegrationTestClient(conn).GetDatabaseInstance(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return &testClient{
		stream: stream,
		respCh: make(chan *pb.GetDatabaseInstanceResponse, 100),
		errCh:  make(chan error, 1),
	}
}

func (c *testClient) Run() {
	defer close(c.respCh)
	for {
		resp, err := c.stream.Recv()
		if err != nil {
			glog.V(2).Infof("Received error from the server: %v", err)
			c.errCh <- err
			break
		}
		c.respCh <- resp
	}
}

func (c *testClient) AssertNoResponse(desc string, t *testing.T) {
	select {
	case r := <-c.respCh:
		t.Fatalf("%v: Got a response, want none: %v", desc, r)
	case err := <-c.errCh:
		t.Fatalf("%v: error: %v", desc, err)
	case <-time.After(50 * time.Millisecond):
	}
}

func (c *testClient) GetResponse(desc string, t *testing.T) (resp *pb.GetDatabaseInstanceResponse) {
	select {
	case resp = <-c.respCh:
	case err := <-c.errCh:
		t.Fatalf("%v: error: %v", desc, err)
	case <-time.After(expMessageDur):
		t.Fatalf("%v: Got no response", desc)
	}
	return
}

func (c *testClient) AssertError(desc string, expErr error, t *testing.T) {
	select {
	case resp := <-c.respCh:
		t.Fatalf("%v: got response, want none: %v", desc, resp)
	case err := <-c.errCh:
		if expErr != nil && err != expErr {
			t.Fatalf("%v: got wrong error (%v), want (%v)", desc, err, expErr)
		}
		glog.V(2).Infof("%v: got expected error: %v", desc, err)
	case <-time.After(expMessageDur):
		t.Fatalf("%v: Got no error, want error", desc)
	}
}

// Wait for Run() to finish. Asserts no errors or responses were received.
func (c *testClient) Wait(t *testing.T) {
	select {
	case r, ok := <-c.respCh:
		if ok {
			t.Fatalf("Wait() got a response, expected channel closed: %v", r)
		}
	case err := <-c.errCh:
		t.Fatalf("Wait() got error: %v", err)
	case <-time.After(time.Second):
		t.Fatal("Wait() did not finish in time")
	}
}
