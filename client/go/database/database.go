// Package database provides a test environment for database integration tests
// written in Golang.
//
// This library expects a database provider instance to already be running.
//
// Methods prefer to panic instead of return error in order to cut down on boilerplate
// in unit tests, and failures in this library should rightfully abort the test anyways.
package database

import (
	"context"
	"flag"

	"github.com/golang/glog"

	"github.com/karagog/db-provider/server/lease"
	pb "github.com/karagog/db-provider/server/proto"
)

// We define flags for passing constructor params in order to facilitate using
// these in unit tests, where we don't want to repeat all the params in every
// test.
var (
	// The default value should be the same as defined in the container's .env file,
	// so it only needs changing under special circumstances.
	databaseProviderAddress = flag.String("db_instance_provider_address", "172.17.0.1:58615",
		"The address where to find the database instance service.")
)

type Instance struct {
	// How to connect, or you can use the Connect/ConnectRoot() convenience methods.
	Info *pb.ConnectionInfo

	lease      *lease.Lease
	lesseeDone chan bool // signals the end of the lessee goroutine
}

// Gets a new instance with the parameters sourced from flags.
// This is the way most tests will get a database instance.
//
// You must Close() it when done to release your lock on the database.
func NewFromFlags(ctx context.Context) *Instance {
	return New(ctx, *databaseProviderAddress)
}

// Gets a database instance from a provider service.
// See also NewFromFlags().
func New(ctx context.Context, databaseAddress string) *Instance {
	// Connect to the test instance service to get a fresh mysql database.
	l, err := lease.Acquire(ctx, databaseAddress)
	if err != nil {
		panic(err)
	}
	i := &Instance{
		lease:      l,
		lesseeDone: make(chan bool),
	}
	go func() {
		defer close(i.lesseeDone)
		i.lease.Maintain()
	}()

	// Block here indefinitely until an instance is ready. The client's Run() method
	// maintains the lease on the instance until our Close() method is called.
	if i.Info, err = i.lease.Wait(ctx); err != nil {
		panic(err)
	}
	glog.V(1).Infof("Lease acquired on %q", i.Info.RootConn.Database)
	return i
}

// Close releases the lock on the database instance when you're done using it.
func (i *Instance) Close() {
	if i.lease == nil {
		return
	}
	glog.V(1).Infof("Returning lease on %q", i.Info.RootConn.Database)
	i.lease.Release()
	i.lease = nil
	<-i.lesseeDone // join our goroutine to avoid leaking
}
