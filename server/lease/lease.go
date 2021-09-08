// Package lease provides a client to the test service that requests and maintains a lease.
package lease

import (
	"context"
	"fmt"
	"io"

	"github.com/golang/glog"
	"google.golang.org/grpc"

	pb "github.com/karagog/db-provider/server/proto"
)

// Halts the program with a log message.
// Can be overridden in tests to avoid actually crashing.
var fatalf = func(msg string, args ...interface{}) { glog.Fatalf(msg, args...) }

// Lease holds and maintains a lease on an instance maintained by the test server.
//
// Create with Acquire(), and then call `go Maintain()` to maintain the lease. It may take a
// while for a lease to be granted, so call Wait() to block until it is.
//
// When you're done with the lease, call Close() to relinquish it.
type Lease struct {
	stream pb.IntegrationTest_GetDatabaseInstanceClient

	ch     chan *pb.ConnectionInfo
	waited bool // already waited?
}

// Requests a new lease from the server. You must also call `go Run()` afterwards.
// Good citizens return the lease explicitly with ReturnLease(), although it will
// be returned automatically when the connection is broken for any reason.
func Acquire(ctx context.Context, serviceAddr string) (*Lease, error) {
	conn, err := grpc.Dial(serviceAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	stream, err := pb.NewIntegrationTestClient(conn).GetDatabaseInstance(ctx)
	if err != nil {
		return nil, err
	}
	if err := stream.Send(&pb.GetDatabaseInstanceRequest{}); err != nil {
		return nil, err
	}
	return &Lease{
		stream: stream,
		ch:     make(chan *pb.ConnectionInfo),
	}, nil
}

// Gives the lease back to the server. You must not use the connection info anymore.
func (l *Lease) Release() {
	l.stream.CloseSend()
}

// Maintain runs the main workflow that maintains the lease with the server.
//
// This should be run in the background via `go Maintain()`. It ends when you return the lease.
func (l *Lease) Maintain() {
	for {
		resp, err := l.stream.Recv()
		if err == io.EOF {
			return
		}
		if err != nil {
			// A sudden loss of the lease is a fatal error that should abort
			// the test program immediately to avoid data corruption.
			fatalf("Halting program due to TestServer error: %v", err)
		}
		if resp.Status != "" {
			glog.V(2).Infof("Received server status: %s", resp.Status)
		}
		if resp.ConnectionInfo == nil {
			continue // server is still processing our request...
		}
		glog.V(2).Infof("Got connection info from the server:\n%v", resp)
		l.ch <- resp.ConnectionInfo
	}
}

// Wait blocks indefinitely until the lease is acquired. This should only be called
// from the main test thread. Calling this multiple times is an error.
func (l *Lease) Wait(ctx context.Context) (*pb.ConnectionInfo, error) {
	if l.waited {
		return nil, fmt.Errorf("Wait() called multiple times")
	}
	l.waited = true
	select {
	case info := <-l.ch:
		return info, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
