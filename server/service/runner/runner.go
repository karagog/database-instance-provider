// Package runner runs the provider service.
package runner

import (
	"net"

	pb "github.com/karagog/db-provider/server/proto"
	"github.com/karagog/db-provider/server/service"
	"google.golang.org/grpc"
)

// Runner runs the service on the given address.
type Runner struct {
	grpcServer *grpc.Server
	service    *service.Service
	listener   net.Listener
	done       chan bool
}

// The address on which to serve.
// E.g. "localhost:0" will grab any available port.
func New(s *service.Service, address string) (*Runner, error) {
	ls, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	r := &Runner{
		service:    s,
		grpcServer: grpc.NewServer(),
		listener:   ls,
		done:       make(chan bool),
	}
	pb.RegisterIntegrationTestServer(r.grpcServer, r.service)
	return r, nil
}

// Runs the grpc service and blocks until finished (which may be forever).
func (r *Runner) Run() {
	if r.done == nil {
		panic("This runner has already been used. Please make a new one.")
	}
	defer close(r.done)
	if err := r.grpcServer.Serve(r.listener); err != nil {
		// The Serve() method should never return except when we shut ourselves down.
		panic(err)
	}
}

// Returns the address at which the service is being provided.
func (r *Runner) Address() string {
	return r.listener.Addr().String()
}

// Stops the service. You must call this when done.
func (s *Runner) Stop() {
	if s.done == nil {
		return // already stopped
	}
	s.grpcServer.Stop()
	<-s.done
	s.done = nil
}
