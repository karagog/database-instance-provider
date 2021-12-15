package mysql

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang/glog"
	pb "github.com/karagog/db-provider/server/proto"
)

// Connects to the database or panics if it fails.
func ConnectOrDie(d *pb.ConnectionDetails) *sql.DB {
	db, err := Connect(d)
	if err != nil {
		panic(err)
	}
	return db
}

// Connects to the database instance using the Mysql driver.
func Connect(d *pb.ConnectionDetails) (*sql.DB, error) {
	connStr := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&multiStatements=true",
		d.User, d.Password, d.Address, d.Port, d.Database)
	glog.V(1).Infof("Using root connection string: %q", connStr)
	return sql.Open("mysql", connStr)
}
