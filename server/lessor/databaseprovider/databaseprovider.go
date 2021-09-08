package databaseprovider

import (
	"context"

	pb "github.com/karagog/db-provider/server/proto"
)

// DatabaseProvider provides database instances for testing.
type DatabaseProvider interface {
	// Creates the database on the server. After calling this you can
	// query the connection info with GetConnectionInfo().
	//
	// This should fail if the database already exists.
	CreateDatabase(context.Context, string) error

	// Drops the database on the server if it exists, otherwise does nothing and returns nil.
	DropDatabase(context.Context, string) error

	// This should be available after creating a database. It tells users how
	// to connect to the given database in this instance.
	GetConnectionInfo(database string) *pb.ConnectionInfo
}
