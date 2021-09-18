package main

import (
	"testing"

	"github.com/go-test/deep"
	pb "github.com/karagog/db-provider/server/proto"
)

func TestConnectionInfo(t *testing.T) {
	m := MysqlProvider{
		Conn: MysqlConnParams{
			User:         "George",
			UserPassword: "1234",
			RootPassword: "5678",
			MysqlAddress: "localhost",
			MysqlPort:    3306,
		},
	}
	ci := m.GetConnectionInfo("mydb")
	diff := deep.Equal(ci, &pb.ConnectionInfo{
		AppConn: &pb.ConnectionDetails{
			User:     "George",
			Password: "1234",
			Address:  "localhost",
			Port:     3306,
			Database: "mydb",
		},
		RootConn: &pb.ConnectionDetails{
			User:     "root",
			Password: "5678",
			Address:  "localhost",
			Port:     3306,
			Database: "mydb",
		},
	})
	if diff != nil {
		t.Fatal(diff)
	}
}
