package backend

import (
	"context"
	"time"

	"github.com/jonboulle/clockwork"
	"gopkg.in/check.v1"
)

type Suite struct {
}

var _ = check.Suite(&Suite{})

func (s *Suite) SetUpSuite(c *check.C) {
}

func (s *Suite) TearDownSuite(c *check.C) {
}

func (s *Suite) TearDownTest(c *check.C) {
}

func (s *Suite) SetUpTest(c *check.C) {
}

func (s *Suite) TestSanitizeBucket(c *check.C) {
	tests := []struct {
		inKey    []byte
		outError bool
	}{
		{
			inKey:    []byte("a-b/c:d/.e_f/01"),
			outError: false,
		},
		{
			inKey:    []byte("/namespaces//params"),
			outError: true,
		},
		{
			inKey:    RangeEnd([]byte("a-b/c:d/.e_f/01")),
			outError: false,
		},
		{
			inKey:    RangeEnd([]byte("/")),
			outError: false,
		},
		{
			inKey:    RangeEnd([]byte("Malformed \xf0\x90\x28\xbc UTF8")),
			outError: true,
		},
	}

	for i, tt := range tests {
		comment := check.Commentf("Test %v, key: %q", i, string(tt.inKey))

		safeBackend := NewSanitizer(&nopBackend{})

		ctx := context.TODO()
		_, err := safeBackend.Get(ctx, tt.inKey)
		c.Assert(err != nil, check.Equals, tt.outError, comment)

		_, err = safeBackend.Create(ctx, Item{Key: tt.inKey})
		c.Assert(err != nil, check.Equals, tt.outError, comment)

		_, err = safeBackend.Put(ctx, Item{Key: tt.inKey})
		c.Assert(err != nil, check.Equals, tt.outError, comment)

		_, err = safeBackend.Update(ctx, Item{Key: tt.inKey})
		c.Assert(err != nil, check.Equals, tt.outError, comment)

		_, err = safeBackend.CompareAndSwap(ctx, Item{Key: tt.inKey}, Item{Key: tt.inKey})
		c.Assert(err != nil, check.Equals, tt.outError, comment)

		err = safeBackend.Delete(ctx, tt.inKey)
		c.Assert(err != nil, check.Equals, tt.outError, comment)

		err = safeBackend.DeleteRange(ctx, tt.inKey, tt.inKey)
		c.Assert(err != nil, check.Equals, tt.outError, comment)
	}
}

type nopBackend struct {
	NoMigrations
}

func (n *nopBackend) Get(_ context.Context, _ []byte) (*Item, error) {
	return &Item{}, nil
}

func (n *nopBackend) GetRange(_ context.Context, startKey []byte, endKey []byte, limit int) (*GetResult, error) {
	return &GetResult{Items: []Item{Item{Key: []byte("foo"), Value: []byte("bar")}}}, nil
}

func (n *nopBackend) Create(_ context.Context, _ Item) (*Lease, error) {
	return &Lease{}, nil
}

func (n *nopBackend) Put(_ context.Context, _ Item) (*Lease, error) {
	return &Lease{}, nil
}

func (n *nopBackend) Update(_ context.Context, _ Item) (*Lease, error) {
	return &Lease{}, nil
}

func (n *nopBackend) CompareAndSwap(_ context.Context, _ Item, _ Item) (*Lease, error) {
	return &Lease{}, nil
}

func (n *nopBackend) Delete(_ context.Context, _ []byte) error {
	return nil
}

func (n *nopBackend) DeleteRange(_ context.Context, _ []byte, _ []byte) error {
	return nil
}

func (n *nopBackend) KeepAlive(_ context.Context, _ Lease, _ time.Time) error {
	return nil
}

func (n *nopBackend) Close() error {
	return nil
}

func (n *nopBackend) Clock() clockwork.Clock {
	return clockwork.NewFakeClock()
}

// NewWatcher returns a new event watcher
func (n *nopBackend) NewWatcher(ctx context.Context, watch Watch) (Watcher, error) {
	return nil, nil
}

// CloseWatchers closes all the watchers
// without closing the backend
func (n *nopBackend) CloseWatchers() {

}
