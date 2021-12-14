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
	"os"

	"github.com/golang/glog"

	"github.com/karagog/db-provider/server/lease"
	pb "github.com/karagog/db-provider/server/proto"
)

type Instance struct {
	// How to connect, or you can use the Connect/ConnectRoot() convenience methods.
	Info *pb.ConnectionInfo

	lease *lease.Lease
}

// Gets a new instance with the parameters sourced from environment variables.
// This is the way most tests will get a database instance.
//
// You must Close() it when done to release your lock on the database.
func NewFromEnv(ctx context.Context) *Instance {
	addr := "172.17.0.1:58615"
	if v := os.Getenv("DB_INSTANCE_PROVIDER_ADDRESS"); v != "" {
		addr = v
	}
	return New(ctx, addr)
}

// Gets a database instance from a provider service.
// See also NewFromEnv().
func New(ctx context.Context, databaseAddress string) *Instance {
	// Connect to the test instance service to get a fresh mysql database.
	l, err := lease.New(ctx, databaseAddress)
	if err != nil {
		panic(err)
	}
	go l.Run()

	// Block here indefinitely until an instance is ready. The client's Run() method
	// maintains the lease on the instance until our Close() method is called.
	i := l.ConnectionInfo()
	glog.V(1).Infof("Lease acquired on %q", i.RootConn.Database)
	return &Instance{
		lease: l,
		Info:  i,
	}
}

// Close releases the lock on the database instance when you're done using it.
func (i *Instance) Close() {
	if i.Info == nil {
		return
	}
	glog.V(1).Infof("Returning lease on %q", i.Info.RootConn.Database)
	i.lease.Close()
	i.Info = nil
}
