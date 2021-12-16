// Package main implements the container application code, which is
// responsible for starting the database and provider services.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/golang/glog"
	"github.com/karagog/clock-go/simulated"
	"github.com/karagog/db-provider/client/go/database/mysql"
	"github.com/karagog/db-provider/server/lessor"
	"github.com/karagog/db-provider/server/service"
	"github.com/karagog/db-provider/server/service/runner"
)

func main() {
	flag.Parse()
	flag.Set("alsologtostderr", "true")

	portStr := getEnvOrDie("PROVIDER_PORT")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		glog.Fatal(err)
	}

	countStr := getEnvOrDie("PROVIDER_DB_INSTANCES")
	count, err := strconv.Atoi(countStr)
	if err != nil {
		glog.Fatal(err)
	}

	// Start up the server.
	svc := &service.Service{
		Clock: simulated.NewClock(time.Now()),
	}
	r, err := runner.New(svc, fmt.Sprintf(":%d", port))
	if err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Starting service on %s", r.Address())
	go r.Run()

	p, err := initContainer(context.Background(),
		getEnvOrDie("MYSQL_ROOT_HOST"),
		getConnectionParamsOrDie())
	if err != nil {
		glog.Fatal(err)
	}
	glog.Info("Database initialized, ready to serve requests...")

	// Now that the database is initialized, update the service which tells
	// clients that it's okay to request databases.
	l := lessor.New(p, count)
	svc.SetLessor(l)

	// Block here indifinitely while the service runs.
	l.Run(context.Background())
}

// initContainer initializes the docker container and returns a provider object.
func initContainer(ctx context.Context, allowConnectionsFrom string, opt *MysqlConnParams) (*MysqlProvider, error) {
	glog.Infof("Initializing mysql database container")

	newCtx, cancel := context.WithDeadline(ctx, time.Now().Add(3*time.Minute))
	defer cancel()

	p := &MysqlProvider{Conn: *opt}
	db, err := mysql.Connect(p.GetConnectionInfo("").RootConn)
	if err != nil {
		return nil, err
	}
	p.DB = db

	err = runMysqlCmd(newCtx, db, fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%s' IDENTIFIED BY '%s'",
		opt.User, allowConnectionsFrom, opt.UserPassword))
	if err != nil {
		return nil, err
	}

	// The backend user can only manipulate rows, not tables (or other things).
	err = runMysqlCmd(newCtx, db,
		fmt.Sprintf("GRANT SELECT, INSERT, UPDATE, DELETE ON *.* TO '%s'@'%s'",
			opt.User, allowConnectionsFrom))
	if err != nil {
		return nil, err
	}

	// Flush privileges to force the settings to take effect.
	if err := runMysqlCmd(newCtx, db, "FLUSH PRIVILEGES"); err != nil {
		return nil, err
	}

	return p, nil
}

// Runs a mysql command, and retries continually until the command succeeds or
// the context is done.
func runMysqlCmd(ctx context.Context, db *sql.DB, cmd string) error {
	glog.V(2).Infof("Running MysqlProvider command: %s", cmd)
	for {
		_, err := db.ExecContext(ctx, cmd)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return fmt.Errorf("gave up retrying connection, last error: %s", err)
		}
		glog.Warningf("Error while trying to connect to MySQL (will retry): %s", err)
		time.Sleep(time.Second)
	}
}

func getEnvOrDie(key string) string {
	ret := os.Getenv(key)
	if ret == "" {
		glog.Fatalf("environment variable missing: %q", key)
	}
	return ret
}

// Gets the connection parameters from the environment.
func getConnectionParamsOrDie() *MysqlConnParams {
	mysqlPort, err := strconv.Atoi(getEnvOrDie("PROVIDER_MYSQL_PORT"))
	if err != nil {
		glog.Fatalf("Invalid mysql port: %s", err)
	}

	return &MysqlConnParams{
		User:         getEnvOrDie("PROVIDER_MYSQL_USER"),
		UserPassword: getEnvOrDie("PROVIDER_MYSQL_USER_PASSWORD"),
		RootPassword: getEnvOrDie("MYSQL_ROOT_PASSWORD"),
		MysqlAddress: getEnvOrDie("PROVIDER_MYSQL_ADDRESS"),
		MysqlPort:    mysqlPort,
	}
}
