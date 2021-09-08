// Package main implements the container application code, which is
// responsible for starting the database and provider services.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"github.com/golang/glog"
	"github.com/karagog/clock-go/simulated"
	"github.com/karagog/db-provider/client/go/database/mysql"
	"github.com/karagog/db-provider/containers/mysql/provider"
	"github.com/karagog/db-provider/server/lessor"
	"github.com/karagog/db-provider/server/service"
	"github.com/karagog/db-provider/server/service/runner"
	"github.com/karagog/shell-go/command"
)

// This identifies the log message that signifies the end of initialization,
// i.e. when its safe to do our own initialization.
var initDoneRegex = regexp.MustCompile(`\[Entrypoint\] Starting MySQL`)

// This is the format for printing redirected output from mysqld.
const logLineFormat = "\t> %s\n"

func main() {
	flag.Parse()
	flag.Set("alsologtostderr", "true")

	p, err := initContainer(context.Background(),
		getEnvWithDefault("APP_MYSQL_ALLOW_CONNECTIONS_FROM", "172.%"),
		getConnectionParamsOrDie())
	if err != nil {
		glog.Fatal(err)
	}

	portStr := getEnvWithDefault("APP_PROVIDER_PORT", "58615")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		glog.Fatal(err)
	}

	countStr := getEnvWithDefault("APP_DB_INSTANCE_COUNT", "20")
	count, err := strconv.Atoi(countStr)
	if err != nil {
		glog.Fatal(err)
	}

	// Start up the server.
	svc := &service.Service{
		Clock:  simulated.NewClock(time.Now()),
		Lessor: lessor.New(p, count),
	}
	go svc.Lessor.Run(context.Background())

	r, err := runner.New(svc, fmt.Sprintf(":%d", port))
	if err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Starting service on %s", r.Address())
	r.Run()
}

// initContainer initializes the docker container and returns a provider object.
func initContainer(ctx context.Context, allowConnectionsFrom string, opt *provider.MysqlConnParams) (*provider.Mysql, error) {
	glog.Infof("Initializing mysql database container")

	// Start up the mysqld service and wait it to initialize.
	initDone := make(chan bool)
	startTime := time.Now()
	if _, err := startMysqld(ctx, initDone); err != nil {
		return nil, err
	}

	// Wait here for the server to initialize. If we try to do our initialization
	// too soon, some of our settings can be overwritten.
	select {
	case <-initDone:
		elapsed := time.Since(startTime)
		glog.Infof("MySQL Server initialized after %v", elapsed)
	case <-time.After(time.Minute):
		// Give up after waiting too long.
		return nil, fmt.Errorf("server did not start")
	}

	newCtx, cancel := context.WithDeadline(ctx, time.Now().Add(3*time.Minute))
	defer cancel()

	err := runMysqlCmd(newCtx,
		fmt.Sprintf("CREATE USER '%s'@'%s' IDENTIFIED BY '%s'",
			opt.User, allowConnectionsFrom, opt.UserPassword))
	if err != nil {
		return nil, err
	}

	// The backend user can only manipulate rows, not tables (or other things).
	err = runMysqlCmd(newCtx,
		fmt.Sprintf("GRANT SELECT, INSERT, UPDATE, DELETE ON *.* TO '%s'@'%s'",
			opt.User, allowConnectionsFrom))
	if err != nil {
		return nil, err
	}

	// Give root unfettered access over the host IP.
	err = runMysqlCmd(newCtx,
		fmt.Sprintf("CREATE USER 'root'@'%s' IDENTIFIED BY '%s'",
			allowConnectionsFrom, opt.RootPassword))
	if err != nil {
		return nil, err
	}
	err = runMysqlCmd(newCtx,
		fmt.Sprintf("GRANT ALL PRIVILEGES ON *.* TO 'root'@'%s'", allowConnectionsFrom))
	if err != nil {
		return nil, err
	}

	// Flush privileges to force the settings to take effect.
	if err := runMysqlCmd(newCtx, "FLUSH PRIVILEGES"); err != nil {
		return nil, err
	}

	p := &provider.Mysql{Conn: *opt}
	p.DB, err = mysql.Connect(p.GetConnectionInfo("").RootConn)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// Starts up mysqld and redirect its output to ours.
func startMysqld(ctx context.Context, initDone chan<- bool) (*exec.Cmd, error) {
	stdoutCh := make(chan string)
	stderrCh := make(chan string)
	c := command.Command{
		// We're inside a mysql-server container, so we can delegate startup to the
		// entry point script.
		Name: "/entrypoint.sh",
		Args: []string{"mysqld"},

		Stdout: stdoutCh,
		Stderr: stderrCh,

		// Initialize with an empty password for root@localhost because it makes setup
		// simpler.
		Env: append(os.Environ(), "MYSQL_ALLOW_EMPTY_PASSWORD=1"),
	}

	// Read the stdout and scan it to know when initialization is done.
	go func() {
		for line := range stdoutCh {
			if initDoneRegex.MatchString(line) {
				close(initDone) // notify that initialization is done
			}
			fmt.Printf(logLineFormat, line)
		}
	}()

	// Read the stderr and report it for logging purposes.
	go func() {
		for line := range stderrCh {
			fmt.Printf(logLineFormat, line)
		}
	}()

	return c.Start(ctx)
}

// Runs a mysql command, and retries continually until the command succeeds or
// the context is done.
func runMysqlCmd(ctx context.Context, cmd string) error {
	glog.V(2).Infof("Running Mysql command: %s", cmd)
	c := command.Command{
		Name: "mysql",
		Args: []string{"-uroot", "-e", cmd},
	}
	for {
		cmd, err := c.Start(ctx)
		if err == nil {
			// The command successfully started, wait for it to finish.
			if err = cmd.Wait(); err == nil {
				return nil
			}
		}
		glog.V(2).Info(err)

		if ctx.Err() != nil {
			return fmt.Errorf("context canceled, last error: %s", err)
		}
		time.Sleep(time.Second)
	}
}

func getEnvWithDefault(key string, def string) string {
	ret := os.Getenv(key)
	if ret == "" {
		return def
	}
	return ret
}

// Gets the connection parameters from the environment.
func getConnectionParamsOrDie() *provider.MysqlConnParams {
	mysqlPort, err := strconv.Atoi(getEnvWithDefault("APP_MYSQL_PUBLISHED_PORT", "53983"))
	if err != nil {
		glog.Fatalf("Invalid mysql port: %s", err)
	}

	return &provider.MysqlConnParams{
		User:         getEnvWithDefault("APP_MYSQL_USER_NAME", "test"),
		UserPassword: getEnvWithDefault("APP_MYSQL_USER_PASSWORD", "test"),
		RootPassword: getEnvWithDefault("APP_MYSQL_ROOT_PASSWORD", "test"),
		MysqlAddress: getEnvWithDefault("APP_MYSQL_CONTAINER_ADDRESS", "172.17.0.1"),
		MysqlPort:    mysqlPort,
	}
}
