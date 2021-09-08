package lease

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/karagog/clock-go/simulated"
	"github.com/karagog/db-provider/server/lessor"
	"github.com/karagog/db-provider/server/lessor/databaseprovider/fake"
	"github.com/karagog/db-provider/server/service"
	"github.com/karagog/db-provider/server/service/runner"
)

func fakeServiceRunner(numInstances int, t *testing.T) *runner.Runner {
	lor := lessor.New(&fake.DatabaseProvider{}, numInstances)
	go lor.Run(context.Background())

	r, err := runner.New(&service.Service{
		Clock:  simulated.NewClock(time.Now()),
		Lessor: lor,
	}, "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestLessee(t *testing.T) {
	r := fakeServiceRunner(1, t)
	go r.Run()
	defer r.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	l, err := Acquire(ctx, r.Address())
	if err != nil {
		t.Fatal(err)
	}
	defer l.Release()

	// Make sure the Run() method finishes.
	done := make(chan bool)
	go func() {
		defer close(done)
		l.Maintain()
	}()

	info, err := l.Wait(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info == nil {
		t.Fatalf("Got nil connection info, want non-nil")
	}
	l.Release()
	<-done
}

func TestWaitContextCanceled(t *testing.T) {
	r := fakeServiceRunner(0, t) // no leases available
	go r.Run()
	defer r.Stop()

	l, err := Acquire(context.Background(), r.Address())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan bool)
	defer func() {
		l.Release()
		<-done // must ensure client is done before stopping the server
	}()
	go func() {
		defer close(done)
		l.Maintain()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Wait in the background because it will block until we cancel the context.
	errCh := make(chan error)
	go func() {
		_, err := l.Wait(ctx)
		errCh <- err
	}()
	cancel()
	err = <-errCh
	if want := context.Canceled; err != want {
		t.Fatalf("Wrong error (%v), want %v", err, want)
	}
}

func TestDoubleWaitError(t *testing.T) {
	r := fakeServiceRunner(1, t)
	go r.Run()
	defer r.Stop()

	l, err := Acquire(context.Background(), r.Address())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan bool)
	defer func() {
		l.Release()
		<-done // must ensure client is done before stopping the server
	}()
	go func() {
		defer close(done)
		l.Maintain()
	}()

	if _, err := l.Wait(context.TODO()); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Wait(context.TODO()); err == nil {
		t.Fatal("Got nil error, want error")
	}
}

// If the server disconnects while we're holding the lease, it should crash us.
func TestServerDisconnectsWhileHoldingLease(t *testing.T) {
	r := fakeServiceRunner(1, t)
	go r.Run()
	defer r.Stop()
	ctx := context.Background()

	l, err := Acquire(ctx, r.Address())
	if err != nil {
		t.Fatal(err)
	}
	defer l.Release()

	// Convert the fatal method to a panic to allow us to recover and avoid actually
	// crashing the test.
	fatalf = func(msg string, args ...interface{}) {
		panic(fmt.Sprintf(msg, args))
	}

	panicCh := make(chan bool) // did Maintain() panic?
	go func() {
		defer func() { panicCh <- recover() != nil }()
		l.Maintain()
	}()

	// Grab and hold a lease.
	if _, err := l.Wait(ctx); err != nil {
		t.Fatal(err)
	}

	// Stop the server, which should disconnect us with an error.
	r.Stop()
	if panicked := <-panicCh; !panicked {
		t.Fatalf("Did not panic, wanted panic")
	}
}

func TestLesseeDialedWrongAddress(t *testing.T) {
	if _, err := Acquire(context.Background(), "localhost:1"); err == nil {
		t.Fatalf("Got nil error, want error")
	}
}
