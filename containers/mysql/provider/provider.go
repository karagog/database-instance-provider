package main

import (
	"context"
	"database/sql"
	"fmt"

	pb "github.com/karagog/db-provider/server/proto"
)

// MysqlConnParams tells us how to connect to a MysqlProvider server.
type MysqlConnParams struct {
	// The user names/passwords for connecting to the MySQL server.
	User         string
	UserPassword string
	RootPassword string

	// The IP address of the MySQL instance.
	MysqlAddress string

	// The port number of the MySQL service.
	MysqlPort int
}

// MysqlProvider implements DatabaseProvider for MysqlProvider databases.
type MysqlProvider struct {
	// Conn tells us how to connect, so we can provide connection info for
	// a database.
	Conn MysqlConnParams

	// DB is needed to create/drop databases.
	DB *sql.DB
}

func (m *MysqlProvider) GetConnectionInfo(database string) *pb.ConnectionInfo {
	return &pb.ConnectionInfo{
		RootConn: &pb.ConnectionDetails{
			User:     "root",
			Password: m.Conn.RootPassword,
			Address:  m.Conn.MysqlAddress,
			Port:     int32(m.Conn.MysqlPort),
			Database: database,
		},
		AppConn: &pb.ConnectionDetails{
			User:     m.Conn.User,
			Password: m.Conn.UserPassword,
			Address:  m.Conn.MysqlAddress,
			Port:     int32(m.Conn.MysqlPort),
			Database: database,
		},
	}
}

func (m *MysqlProvider) CreateDatabase(ctx context.Context, name string) error {
	_, err := m.DB.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %s", name))
	return err
}

func (m *MysqlProvider) DropDatabase(ctx context.Context, name string) error {
	_, err := m.DB.ExecContext(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", name))
	return err
}
