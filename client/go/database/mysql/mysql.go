package mysql

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
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
	return sql.Open("mysql",
		fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&multiStatements=true",
			d.User, d.Password, d.Address, d.Port, d.Database))
}
