/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sqlbk

import (
	"bytes"
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
)

// New returns a Backend that uses a driver to communicate with the database.
// A non-nil error means the connection pool is ready and the database has been
// migrated to the most recent version.
func New(ctx context.Context, driver Driver) (*Backend, error) {
	bk, err := newWithConfig(ctx, driver, driver.Config())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = bk.start(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return bk, nil
}

// newWithConfig opens a connection to the database and returns an initialized
// Backend instance. Background processes have not been started.
func newWithConfig(ctx context.Context, driver Driver, cfg *Config) (*Backend, error) {
	db, err := driver.Open(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	bk := &Backend{
		Config: cfg,
		db:     db,
		buf:    backend.NewCircularBuffer(backend.BufferCapacity(cfg.BufferSize)),
		bgDone: make(chan struct{}),
	}
	bk.closeCtx, bk.closeFn = context.WithCancel(context.Background())
	return bk, nil
}

// Backend implements a storage backend for SQL databases.
type Backend struct {
	*Config
	db  DB
	buf *backend.CircularBuffer

	closed   int32 // atomic
	closeCtx context.Context
	closeFn  context.CancelFunc
	bgDone   chan struct{}
}

// Close the backend.
func (b *Backend) Close() error {
	if !atomic.CompareAndSwapInt32(&b.closed, 0, 1) {
		return nil
	}
	b.closeFn()
	select {
	case <-b.bgDone:
	case <-time.After(time.Second * 10):
	}
	return trace.NewAggregate(b.buf.Close(), b.db.Close())
}

// NewWatcher returns a new event watcher.
func (b *Backend) NewWatcher(ctx context.Context, watch backend.Watch) (backend.Watcher, error) {
	return b.buf.NewWatcher(ctx, watch)
}

// Clock returns the clock used by this backend.
func (b *Backend) Clock() clockwork.Clock {
	return b.Config.Clock
}

// CloseWatchers closes all event watchers without closing the backend.
func (b *Backend) CloseWatchers() {
	b.buf.Clear()
}

// retryTx retries a transaction when it results in an ErrRetry error.
// Failed transactions are more likely to occur when the transaction isolation
// level of the database is serializable.
//
// Callers supply a begin function to create a new transaction, which creates
// either a read/write or read-only transaction. Delays between retries is
// controlled by setting the RetryDelayPeriod configuration variable. The
// amount of time delayed is passed through a jitter algorithm. And the total
// amount of time allocated for retries is defined by RetryTimeout.
//
// Returning an error from txFn will rollback the transaction and stop retries.
func (b *Backend) retryTx(ctx context.Context, begin func(context.Context) Tx, txFn func(tx Tx) error) error {
	ctx, cancel := context.WithTimeout(ctx, b.RetryTimeout)
	defer cancel()

	var delay *utils.Linear
	tx := begin(ctx)
	for {
		if tx.Err() != nil {
			return tx.Err()
		}

		err := txFn(tx)
		switch {
		case err != nil:
			return tx.Rollback(err)

		case tx.Commit() == nil:
			return nil

		case !errors.Is(tx.Err(), ErrRetry):
			return tx.Err()
		}

		// Retry transaction after delay.
		if delay == nil {
			retryDelayPeriod := b.RetryDelayPeriod
			if retryDelayPeriod == 0 { // sanity check (0 produces an error in NewLinear)
				retryDelayPeriod = DefaultRetryDelayPeriod
			}
			delay, err = utils.NewLinear(utils.LinearConfig{
				First:  retryDelayPeriod,
				Step:   retryDelayPeriod,
				Max:    retryDelayPeriod,
				Jitter: utils.NewJitter(),
			})
			if err != nil {
				return trace.BadParameter("[BUG] invalid retry delay configuration: %v", err)
			}
		}
		select {
		case <-delay.After():
			tx = begin(ctx)
			delay.Inc()

		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		}
	}
}

// Create backend item if it does not exist. A put event is emitted if the item
// is created without error.
func (b *Backend) Create(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	if len(item.Key) == 0 {
		return nil, trace.BadParameter("missing parameter key")
	}
	var lease backend.Lease
	err := b.retryTx(ctx, b.db.Begin, func(tx Tx) error {
		if tx.LeaseExists(item.Key) {
			return trace.AlreadyExists("backend item already exists for %v", string(item.Key))
		}
		item.ID = tx.InsertItem(item)
		tx.UpsertLease(item)
		tx.InsertEvent(types.OpPut, item)
		lease = newLease(item)
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &lease, nil
}

// Put creates or updates a backend item. A put event is emitted if the item is
// created without error.
func (b *Backend) Put(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	if len(item.Key) == 0 {
		return nil, trace.BadParameter("missing parameter key")
	}
	var lease backend.Lease
	err := b.retryTx(ctx, b.db.Begin, func(tx Tx) error {
		item.ID = tx.InsertItem(item)
		tx.UpsertLease(item)
		tx.InsertEvent(types.OpPut, item)
		lease = newLease(item)
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &lease, nil
}

// PutRange creates or updates a list of backend items. The batch operation is
// all-or-none. A put event is emitted for each item if the entire batch is successful.
func (b *Backend) PutRange(ctx context.Context, items []backend.Item) error {
	if len(items) == 0 {
		return nil
	}
	return b.retryTx(ctx, b.db.Begin, func(tx Tx) error {
		for _, item := range items {
			item.ID = tx.InsertItem(item)
			tx.UpsertLease(item)
			tx.InsertEvent(types.OpPut, item)
			if tx.Err() != nil {
				return nil
			}
		}
		return nil
	})
}

// CompareAndSwap replaces a backend item if the existing item has an expected
// value. A trace.CompareFailed error is returned when the item does not exist
// or the current item's value is not equal to the expected value. A put event
// is emitted if the operation succeeds without error.
func (b *Backend) CompareAndSwap(ctx context.Context, expected, replaceWith backend.Item) (*backend.Lease, error) {
	if len(expected.Key) == 0 {
		return nil, trace.BadParameter("missing parameter Key")
	}
	if len(replaceWith.Key) == 0 {
		return nil, trace.BadParameter("missing parameter Key")
	}
	if !bytes.Equal(expected.Key, replaceWith.Key) {
		return nil, trace.BadParameter("expected and replaceWith keys should match")
	}

	var lease backend.Lease
	err := b.retryTx(ctx, b.db.Begin, func(tx Tx) error {
		value := tx.GetItemValue(expected.Key)
		if tx.Err() != nil {
			if errors.Is(tx.Err(), ErrNotFound) {
				return trace.CompareFailed("backend item does not exist for key %q", string(expected.Key))
			}
			return nil
		}
		if !bytes.Equal(value, expected.Value) {
			return trace.CompareFailed("current value does not match expected for %v", string(expected.Key))
		}
		replaceWith.ID = tx.InsertItem(replaceWith)
		tx.UpsertLease(replaceWith)
		tx.InsertEvent(types.OpPut, replaceWith)
		lease = newLease(replaceWith)
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &lease, nil
}

// Update an existing backend item. A put event is emitted if the item is
// updated without error.
func (b *Backend) Update(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	if len(item.Key) == 0 {
		return nil, trace.BadParameter("missing parameter key")
	}
	var lease backend.Lease
	err := b.retryTx(ctx, b.db.Begin, func(tx Tx) error {
		item.ID = tx.InsertItem(item)
		tx.UpdateLease(item)
		tx.InsertEvent(types.OpPut, item)
		lease = newLease(item)
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, trace.NotFound("backend item does not exist for key %q", string(item.Key))
		}
		return nil, trace.Wrap(err)
	}
	return &lease, nil
}

// Get a backend item.
func (b *Backend) Get(ctx context.Context, key []byte) (*backend.Item, error) {
	if len(key) == 0 {
		return nil, trace.BadParameter("missing parameter key")
	}

	var item *backend.Item
	err := b.retryTx(ctx, b.db.ReadOnly, func(tx Tx) error {
		item = tx.GetItem(key)
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, trace.NotFound("backend item does not exist for key %q", string(key))
		}
		return nil, trace.Wrap(err)
	}
	return item, nil
}

// GetRange returns a list of backend items whose key is inclusively between startKey and endKey.
// DefaultRangeLimit is used when limit is zero.
func (b *Backend) GetRange(ctx context.Context, startKey, endKey []byte, limit int) (*backend.GetResult, error) {
	if len(startKey) == 0 {
		return nil, trace.BadParameter("missing parameter startKey")
	}
	if len(endKey) == 0 {
		return nil, trace.BadParameter("missing parameter endKey")
	}
	if limit <= 0 {
		limit = backend.DefaultRangeLimit
	}

	var items []backend.Item
	err := b.retryTx(ctx, b.db.ReadOnly, func(tx Tx) error {
		items = tx.GetItemRange(startKey, endKey, limit)
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, trace.NotFound("backend items do not exist for key range: %q - %q", string(startKey), string(endKey))
		}
		return nil, trace.Wrap(err)
	}
	return &backend.GetResult{Items: items}, nil
}

// Delete a backend item. A delete event is emitted if the item existed and
// was deleted without error.
func (b *Backend) Delete(ctx context.Context, key []byte) error {
	if len(key) == 0 {
		return trace.BadParameter("missing parameter key")
	}

	err := b.retryTx(ctx, b.db.Begin, func(tx Tx) error {
		id := tx.DeleteLease(key)
		tx.InsertEvent(types.OpDelete, backend.Item{Key: key, ID: id})
		return nil
	})
	if errors.Is(err, ErrNotFound) {
		return trace.NotFound("backend item does not exist for key %q", string(key))
	}
	return trace.Wrap(err)
}

// DeleteRange deletes all backend items whose key is inclusively between
// startKey and endKey. Delete events are emitted for all deleted items.
func (b *Backend) DeleteRange(ctx context.Context, startKey, endKey []byte) error {
	if len(startKey) == 0 {
		return trace.BadParameter("missing parameter startKey")
	}
	if len(endKey) == 0 {
		return trace.BadParameter("missing parameter endKey")
	}

	err := b.retryTx(ctx, b.db.Begin, func(tx Tx) error {
		items := tx.DeleteLeaseRange(startKey, endKey)
		for _, item := range items {
			tx.InsertEvent(types.OpDelete, item)
		}
		return nil
	})
	if errors.Is(err, ErrNotFound) {
		return trace.NotFound("backend items do not exist for key range: %q - %q", string(startKey), string(endKey))
	}
	return trace.Wrap(err)
}

// KeepAlive updates expiry for a backend item. A put event is emitted if the
// backend item was updated without error.
func (b *Backend) KeepAlive(ctx context.Context, lease backend.Lease, expires time.Time) error {
	if len(lease.Key) == 0 {
		return trace.BadParameter("lease key is not specified")
	}

	item := backend.Item{
		Key:     lease.Key,
		ID:      lease.ID,
		Expires: expires,
	}
	err := b.retryTx(ctx, b.db.Begin, func(tx Tx) error {
		tx.UpdateLease(item)
		tx.InsertEvent(types.OpPut, item)
		return nil
	})
	if errors.Is(err, ErrNotFound) {
		return trace.NotFound("backend item does not exist for key %q", string(item.Key))
	}
	return trace.Wrap(err)
}

// now returns the current clock time.
func (b *Backend) now() time.Time {
	return b.Config.Clock.Now()
}

// newLease returns a backend lease for the backend item.
// An empty lease is returned when the backend item never expires.
func newLease(item backend.Item) backend.Lease {
	if item.Expires.IsZero() {
		return backend.Lease{}
	}
	return backend.Lease{Key: item.Key, ID: item.ID}
}
