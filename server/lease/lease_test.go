package lease

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-test/deep"
	"github.com/karagog/clock-go/simulated"
	"github.com/karagog/db-provider/server/lessor"
	"github.com/karagog/db-provider/server/lessor/databaseprovider/fake"
	"github.com/karagog/db-provider/server/service"
	"github.com/karagog/db-provider/server/service/runner"
)

// Starts up a fake database provider service in-memory.
func fakeServiceRunner(numInstances int, t *testing.T) *runner.Runner {
	l := lessor.New(&fake.DatabaseProvider{}, numInstances)
	go l.Run(context.Background())

	svc := &service.Service{
		Clock: simulated.NewClock(time.Now()),
	}
	svc.SetLessor(l)
	r, err := runner.New(svc, "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestLease(t *testing.T) {
	r := fakeServiceRunner(1, t)
	go r.Run()
	defer r.Stop()

	l, err := New(context.Background(), r.Address())
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// Call Run() and make sure it finishes.
	done := make(chan bool)
	go func() {
		defer close(done)
		l.Run()
	}()

	// Get the connection info, which blocks until we get a lease.
	info := l.ConnectionInfo()
	if info == nil {
		t.Fatalf("Got nil connection info, want non-nil")
	}

	// Check that the result was cached by calling it again.
	info2 := l.ConnectionInfo()
	if diff := deep.Equal(info2, info); diff != nil {
		t.Fatal(diff)
	}

	// Close down the lease and ensure that Run() finished.
	l.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run() never completed")
	}
}

// If the server disconnects while we're holding the lease, it should crash us.
func TestServerDisconnectsWhileHoldingLease(t *testing.T) {
	r := fakeServiceRunner(1, t)
	go r.Run()
	defer r.Stop()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	l, err := New(ctx, r.Address())
	if err != nil {
		t.Fatal(err)
	}

	// Convert the fatal method to a panic to allow us to recover and avoid actually
	// crashing the test.
	fatalf = func(msg string, args ...interface{}) {
		panic(fmt.Sprintf(msg, args))
	}

	panicCh := make(chan bool) // did the goroutine panic?
	go func() {
		defer func() { panicCh <- recover() != nil }()
		l.Run()
	}()

	// Grab and hold a lease.
	if l.ConnectionInfo() == nil {
		t.Fatal("Got nil connection info, want info")
	}

	// Stop the server, which should disconnect us with an error.
	r.Stop()
	if panicked := <-panicCh; !panicked {
		t.Fatalf("Did not panic, wanted panic")
	}
}

func TestLesseeDialedWrongAddress(t *testing.T) {
	if _, err := New(context.Background(), "localhost:1"); err == nil {
		t.Fatalf("Got nil error, want error")
	}
}
