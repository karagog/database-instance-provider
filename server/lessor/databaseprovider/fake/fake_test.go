package fake

import (
	"context"
	"errors"
	"testing"

	"github.com/go-test/deep"

	"github.com/karagog/db-provider/server/lessor/databaseprovider"
	pb "github.com/karagog/db-provider/server/proto"
)

// Make sure the fake always implements the provider interface.
func TestSatisfiesInterface(t *testing.T) {
	func(databaseprovider.DatabaseProvider) {}(&DatabaseProvider{})
}

func TestDatabaseProvider(t *testing.T) {
	p := DatabaseProvider{
		CreateErr: errors.New("create"),
		DropErr:   errors.New("drop"),
		Info:      pb.ConnectionInfo{},
	}
	ctx := context.Background()

	name1 := "joe"
	if got, want := p.CreateDatabase(ctx, name1), p.CreateErr; got != want {
		t.Fatalf("Got error %q, want %q", got, want)
	}

	if diff := deep.Equal(p.CreateList, []string{name1}); diff != nil {
		t.Fatal(diff)
	}
	if len(p.DropList) != 0 {
		t.Fatalf("DropList was populated, want no items")
	}

	p.CreateList = nil
	if got, want := p.DropDatabase(ctx, name1), p.DropErr; got != want {
		t.Fatalf("Got error %q, want %q", got, want)
	}

	if len(p.CreateList) != 0 {
		t.Fatalf("DropList was populated, want no items")
	}
	if diff := deep.Equal(p.DropList, []string{name1}); diff != nil {
		t.Fatal(diff)
	}

	if got, want := p.GetConnectionInfo(name1), &p.Info; got != want {
		t.Fatalf("Got %v, want %v", got, want)
	}
}
