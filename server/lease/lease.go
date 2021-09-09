// Package lease provides a client to the test service that requests and maintains a lease.
package lease

import (
	"context"
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
// Create with Acquire(), and then call `go Run()` to maintain the lease. It may take a
// while for a lease to be granted, so call Wait() to block until it is.
//
// When you're done with the lease, call Close() to relinquish it.
type Lease struct {
	stream   pb.IntegrationTest_GetDatabaseInstanceClient
	ch       chan *pb.ConnectionInfo
	connInfo *pb.ConnectionInfo
}

// Requests a new lease from the server. You must call 'go Run()' before using.
// Good citizens return the lease explicitly by calling Close(),
// although it will be returned automatically when the connection is
// broken for any reason.
func New(ctx context.Context, serviceAddr string) (*Lease, error) {
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

// Run runs the main workflow that maintains the lease with the server.
// It returns then the lease context is finished, or if it loses
// connection to the database provider service.
//
// This ends when you return the lease, or there's an error from the server.
func (l *Lease) Run() {
	defer close(l.ch)
	for {
		resp, err := l.stream.Recv()
		if err != nil {
			if err == io.EOF {
				return
			}
			// A sudden loss of the lease is a fatal error that should abort
			// the test program immediately to avoid conflicting with another test.
			fatalf("Halting program due loss of lease on the test database: %v", err)
		}
		if resp.Status != "" {
			glog.V(1).Infof("Received server status: %s", resp.Status)
		}
		if resp.ConnectionInfo == nil {
			continue // server is still processing our request...
		}
		glog.V(1).Infof("Got connection info from the server:\n%v", resp)
		l.ch <- resp.ConnectionInfo
	}
}

// Close closes the object and releases the lease. This must be called
// when you're done with it.
func (l *Lease) Close() {
	l.connInfo = nil
	l.stream.CloseSend()
	<-l.ch // join the goroutine method
}

// ConnectionInfo returns the connection info to the database on
// which it holds a lease. It blocks indefinitely until the lease is acquired
// and the connection info is available. The result is cached, so
// subsequent calls return immediately.
func (l *Lease) ConnectionInfo() *pb.ConnectionInfo {
	if l.connInfo == nil {
		l.connInfo = <-l.ch
	}
	return l.connInfo
}
