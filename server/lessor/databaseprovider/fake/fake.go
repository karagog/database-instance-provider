// Package fake implements a fake database provider for testing.
package fake

import (
	"context"

	pb "github.com/karagog/db-provider/server/proto"
)

// DatabaseProvider is a fake database provider that returns whatever you tell it.
type DatabaseProvider struct {
	CreateList []string // A list of all calls to CreateDatabase.
	CreateErr  error

	DropList []string // A list of all calls to DropDatabase.
	DropErr  error

	Info pb.ConnectionInfo
}

func (p *DatabaseProvider) CreateDatabase(ctx context.Context, name string) error {
	p.CreateList = append(p.CreateList, name)
	return p.CreateErr
}

func (p *DatabaseProvider) DropDatabase(ctx context.Context, name string) error {
	p.DropList = append(p.DropList, name)
	return p.DropErr
}

func (p *DatabaseProvider) GetConnectionInfo(database string) *pb.ConnectionInfo {
	return &p.Info
}
