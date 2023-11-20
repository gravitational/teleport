//go:build !cgo

package lite

import (
	"context"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/trace"
	"time"
)

// New returns a new instance of sqlite backend
func New(ctx context.Context, params backend.Params) (*Backend, error) {
	return nil, trace.NotImplemented("lite backend requires cgo")
}

// NewWithConfig returns a new instance of lite backend using
// configuration struct as a parameter
func NewWithConfig(ctx context.Context, cfg Config) (*Backend, error) {
	return nil, trace.NotImplemented("lite backend requires cgo")
}

func (l *Backend) Create(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	return nil, trace.NotImplemented("lite backend requires cgo")
}

func (l *Backend) CompareAndSwap(ctx context.Context, expected backend.Item, replaceWith backend.Item) (*backend.Lease, error) {
	return nil, trace.NotImplemented("lite backend requires cgo")
}

func (l *Backend) Put(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	return nil, trace.NotImplemented("lite backend requires cgo")
}

func (l *Backend) Imported(ctx context.Context) (imported bool, err error) {
	return false, trace.NotImplemented("lite backend requires cgo")
}

func (l *Backend) Import(ctx context.Context, items []backend.Item) error {
	return trace.NotImplemented("lite backend requires cgo")
}

func (l *Backend) PutRange(ctx context.Context, items []backend.Item) error {
	return trace.NotImplemented("lite backend requires cgo")
}

func (l *Backend) Update(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	return nil, trace.NotImplemented("lite backend requires cgo")
}

func (l *Backend) Get(ctx context.Context, key []byte) (*backend.Item, error) {
	return nil, trace.NotImplemented("lite backend requires cgo")
}

func (l *Backend) GetRange(ctx context.Context, startKey []byte, endKey []byte, limit int) (*backend.GetResult, error) {
	return nil, trace.NotImplemented("lite backend requires cgo")
}

func (l *Backend) KeepAlive(ctx context.Context, lease backend.Lease, expires time.Time) error {
	return trace.NotImplemented("lite backend requires cgo")
}

func (l *Backend) Delete(ctx context.Context, key []byte) error {
	return trace.NotImplemented("lite backend requires cgo")
}

func (l *Backend) DeleteRange(ctx context.Context, startKey, endKey []byte) error {
	return trace.NotImplemented("lite backend requires cgo")
}

func (l *Backend) NewWatcher(ctx context.Context, watch backend.Watch) (backend.Watcher, error) {
	return nil, trace.NotImplemented("lite backend requires cgo")
}

func (l *Backend) ConditionalUpdate(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	return nil, trace.NotImplemented("lite backend requires cgo")
}

func (l *Backend) ConditionalDelete(ctx context.Context, key []byte, revision string) error {
	return trace.NotImplemented("lite backend requires cgo")
}

func (l *Backend) CloseWatchers() {}

// Close closes all associated resources
func (l *Backend) Close() error {
	return trace.NotImplemented("lite backend requires cgo")
}
