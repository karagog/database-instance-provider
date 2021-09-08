package mysql

import (
	"fmt"
	"regexp"
	"strconv"
	"testing"

	sqle "github.com/dolthub/go-mysql-server"
	"github.com/dolthub/go-mysql-server/auth"
	"github.com/dolthub/go-mysql-server/memory"
	"github.com/dolthub/go-mysql-server/server"

	pb "github.com/karagog/db-provider/server/proto"
)

func TestConnect(t *testing.T) {
	user := "user"
	userPassword := "asdf"
	mysqlAddress := "localhost"
	databaseName := "test"

	// Initialize an in-memory mysql database to connect to.
	dvr := sqle.NewDefault()
	dvr.AddDatabase(memory.NewDatabase(databaseName))

	config := server.Config{
		Protocol: "tcp",
		Address:  fmt.Sprintf("%s:0", mysqlAddress), // find a free open port
		Auth:     auth.NewNativeSingle(user, userPassword, auth.AllPermissions),
	}
	s, err := server.NewDefaultServer(config, dvr)
	if err != nil {
		t.Fatal(err)
	}
	doneCh := make(chan bool)
	go func() {
		defer close(doneCh)
		if err := s.Start(); err != nil {
			panic(err)
		}
	}()

	re := regexp.MustCompile(`.*:(\d+)`)
	groups := re.FindStringSubmatch(s.Listener.Addr().String())
	if groups == nil {
		t.Fatal("Unable to find port number")
	}

	port, err := strconv.Atoi(groups[1])
	if err != nil {
		t.Fatal(err)
	}
	db := ConnectOrDie(&pb.ConnectionDetails{
		User:     user,
		Password: userPassword,
		Address:  mysqlAddress,
		Port:     int32(port),
		Database: databaseName,
	})
	if err := db.Ping(); err != nil {
		t.Fatalf("Unable to ping database: %s", err)
	}
}
