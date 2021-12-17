package runner

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/karagog/clock-go/simulated"
	"github.com/karagog/db-provider/server/lessor"
	"github.com/karagog/db-provider/server/lessor/databaseprovider/fake"
	pb "github.com/karagog/db-provider/server/proto"
	"github.com/karagog/db-provider/server/service"
	"google.golang.org/grpc"
)

func TestRunner(t *testing.T) {
	// Initialize a fake service for testing.
	svc := service.New(simulated.NewClock(time.Now()))
	l := lessor.New(&fake.DatabaseProvider{}, 1)
	svc.SetLessor(l)
	ctx := context.Background()
	go l.Run(ctx)

	// Create a new runner object, to provide the service on a random local port.
	r, err := New(svc, "localhost:0")
	if err != nil {
		t.Fatal(err)
	}

	// The address field should be populated immediately after creation.
	addr := r.Address()
	re := regexp.MustCompile(`.*?:\d+`)
	if !re.MatchString(addr) {
		t.Fatalf("Got address %q, want match %q", addr, re)
	}

	// We should be unable to run another instance on the same address.
	if _, err = New(svc, addr); err == nil {
		t.Fatal("Got nil error, want error")
	}

	// Dial the service before it has started running, which grpc takes care
	// to retry for us.
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}

	// Try calling an RPC before Run(), it should not work.
	timeout, cancelTimeout := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancelTimeout()
	_, err = pb.NewIntegrationTestClient(conn).GetDatabaseInstance(timeout)
	if err == nil {
		t.Fatalf("GRPC service started early, want it to start after Run()")
	}

	// Start the service running in the background, with a channel to detect
	// when it's done.
	runDone := make(chan bool)
	go func() {
		defer close(runDone)
		r.Run()
	}()

	// Initiate an RPC to make sure it's up. We're not really interested
	// in the details of the request/response, only that the RPC worked.
	stream, err := pb.NewIntegrationTestClient(conn).GetDatabaseInstance(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.Send(&pb.GetDatabaseInstanceRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatal("Got error, want response message")
	}

	// Make sure the Run() method is still running until we Stop() it.
	select {
	case <-runDone:
		t.Fatal("Run() finished early")
	case <-time.After(time.Millisecond):
	}

	// Stop the service and make sure the Run() method completes.
	r.Stop()
	select {
	case <-runDone:
	case <-time.After(time.Second):
		t.Fatal("Run() didn't complete")
	}

	// Calling stop a second time is okay (it does nothing).
	r.Stop()

	// Calling Run() after stopping is an error.
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("Got nil panic, want panic")
			}
		}()
		r.Run()
	}()
}
