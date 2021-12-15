// Healthcheck reports the health of the database provider service in a container.
// It queries the status RPC and returns 0 if the service is ready for requests,
// or 1 otherwise.
package main

import (
	"context"
	"flag"
	"os"

	"github.com/golang/glog"
	pb "github.com/karagog/db-provider/server/proto"
	"google.golang.org/grpc"
)

var serviceAddr = flag.String("address", "localhost:58615", "The provider service address")

func main() {
	conn, err := grpc.Dial(*serviceAddr, grpc.WithInsecure())
	if err != nil {
		glog.Infof("Service unreachable: %s", err)
		os.Exit(1)
	}

	cli := pb.NewIntegrationTestClient(conn)
	resp, err := cli.GetStatus(context.Background(), &pb.GetStatusRequest{})
	if err != nil {
		glog.Infof("Service unreachable: %s", err)
		os.Exit(1)
	}

	switch resp.State {
	case pb.GetStatusResponse_UP:
		os.Exit(0) // service ready
	default:
		glog.Infof("Service not yet ready: %s", pb.GetStatusResponse_State_name[int32(resp.State)])
		os.Exit(1)
	}
}
