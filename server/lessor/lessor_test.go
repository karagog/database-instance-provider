package lessor

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/karagog/db-provider/server/lessor/databaseprovider/fake"

	"github.com/go-test/deep"
)

func TestLessor(t *testing.T) {
	p := &fake.DatabaseProvider{}
	les := New(p, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan bool)
	go func() {
		defer close(done)
		les.Run(ctx)
	}()

	// Grab a lease and test the database connection.
	l, err := les.Lease(ctx)
	if err != nil {
		t.Fatalf("Lease failed: %v", err)
	}
	info := les.ConnectionInfo(l)
	if info != &p.Info {
		t.Fatalf("Got info %v, want %v", info, &p.Info)
	}

	if got, want := len(p.CreateList), 1; got != want {
		t.Fatalf("Create called %v times, want %v", got, want)
	}
	if diff := deep.Equal(p.CreateList, p.DropList); diff != nil {
		t.Fatalf("Drop and Create were not called evenly. Want the database to be dropped and created: %v", diff)
	}
	p.CreateList = nil
	p.DropList = nil

	// Try to grab a second lease, which should block.
	ctx2, cancelTimeout := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancelTimeout()
	if _, err := les.Lease(ctx2); err != context.DeadlineExceeded {
		t.Fatalf("Got error (%v), want (%v)", err, context.DeadlineExceeded)
	}

	if p.CreateList != nil {
		t.Fatal("Create called, want not called")
	}
	if p.DropList != nil {
		t.Fatal("Drop called, want not called")
	}

	// Return the lease as we're done with the database.
	les.Return(l)

	// Grab another lease, which should give us a fresh database without the changes from earlier.
	ctx2, cancelTimeout = context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelTimeout()
	if _, err = les.Lease(ctx2); err != nil {
		t.Fatalf("Error getting second lease: %v", err)
	}

	// Expect that the database was reset in between leases.
	if got, want := len(p.CreateList), 1; got != want {
		t.Fatalf("Create called %v times, want %v", got, want)
	}
	if diff := deep.Equal(p.CreateList, p.DropList); diff != nil {
		t.Fatalf("Drop and Create were not called evenly. Want the database to be dropped and created: %v", diff)
	}

	// Close down and make sure the lessor finishes.
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Lessor Run() did not finish")
	}
}

func TestDropDatabaseError(t *testing.T) {
	p := &fake.DatabaseProvider{DropErr: fmt.Errorf("Oof!")}
	les := New(p, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go les.Run(ctx)

	ctx2, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()
	if _, err := les.Lease(ctx2); err != context.DeadlineExceeded {
		t.Fatal(err)
	}
}

func TestCreateDatabaseError(t *testing.T) {
	p := &fake.DatabaseProvider{CreateErr: fmt.Errorf("Oof!")}
	les := New(p, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go les.Run(ctx)

	ctx2, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()
	if _, err := les.Lease(ctx2); err != context.DeadlineExceeded {
		t.Fatal(err)
	}
}

func TestReturnInvalidLease(t *testing.T) {
	p := &fake.DatabaseProvider{}
	les := New(p, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go les.Run(ctx)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Did not panic")
		}
	}()
	les.Return(Lease("invalid")) // this lease did not come from a call to Lease()
}
