package lessor

import (
	"context"
	"fmt"
	"sync"

	"github.com/golang/glog"

	"github.com/karagog/db-provider/server/lessor/databaseprovider"
	pb "github.com/karagog/db-provider/server/proto"
)

// Lease is an opaque handle for referencing your instance lease.
type Lease interface{}

type Lessor struct {
	numDB   int // const
	readyCh chan string
	resetCh chan string

	provider  databaseprovider.DatabaseProvider
	databases map[string]bool
}

// We will set up and manage this many databases.
func New(p databaseprovider.DatabaseProvider, numDB int) *Lessor {
	return &Lessor{
		provider:  p,
		numDB:     numDB,
		readyCh:   make(chan string, numDB),
		resetCh:   make(chan string, numDB),
		databases: make(map[string]bool),
	}
}

func (l *Lessor) Run(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(l.numDB)
	for i := 0; i < l.numDB; i++ {
		// Spawn a pool of workers to reset databases in parallel.
		go func() {
			defer wg.Done()
			l.resetWorker(ctx)
		}()

		// Create a new instance handle and pass it to the reset worker.
		name := fmt.Sprintf("testserver_db_%d", i)
		l.databases[name] = true
		l.resetCh <- name
	}
	wg.Wait()
}

func (l *Lessor) resetWorker(ctx context.Context) {
	for {
		select {
		case name := <-l.resetCh:
			if err := l.reset(ctx, name); err != nil {
				glog.Errorf("Dropping database %s due to error: %s", name, err)
			}
		case <-ctx.Done():
			return
		}
	}
}

// Blocks until a lease is granted, or the context has ended.
func (l *Lessor) Lease(ctx context.Context) (Lease, error) {
	glog.V(2).Infof("Lease called")
	select {
	case l := <-l.readyCh:
		glog.V(2).Infof("Handing out lease on %q", l)
		return l, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (l *Lessor) ConnectionInfo(lease Lease) *pb.ConnectionInfo {
	return l.provider.GetConnectionInfo(lease.(string))
}

func (l *Lessor) Return(lease Lease) {
	glog.V(2).Infof("Return called on lease: '%s'", lease)
	name := lease.(string)
	if _, ok := l.databases[name]; !ok {
		panic(fmt.Sprintf("Invalid lease: %v", lease))
	}
	l.resetCh <- name
}

func (l *Lessor) reset(ctx context.Context, database string) error {
	if err := l.provider.DropDatabase(ctx, database); err != nil {
		return err
	}
	if err := l.provider.CreateDatabase(ctx, database); err != nil {
		return err
	}
	l.readyCh <- database
	return nil
}
