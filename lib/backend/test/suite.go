/*
Copyright 2015-2021 Gravitational, Inc.

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

// Package test contains a backend acceptance test suite that is backend implementation independent
// each backend will use the suite to test itself
package test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
)

var (
	ErrMirrorNotSupported           = errors.New("mirror mode not supported")
	ErrConcurrentAccessNotSupported = errors.New("concurrent access not supported")
)

type ConstructionOptions struct {
	MirrorMode bool

	// ConcurrentBackend indicates that the Backend Constructor function should not
	// create an entirely independent data store, but instead should create a
	// new interface to the same underlying data store as `ConcurrentBackend`.
	ConcurrentBackend backend.Backend
}

// ApplyOptions constructs a new `ConstructionOptions` value from a
// sensible default and then applies the supplied options to it.
func ApplyOptions(options []ConstructionOption) (*ConstructionOptions, error) {
	result := ConstructionOptions{
		MirrorMode: false,
	}
	if err := result.Apply(options); err != nil {
		return nil, err
	}
	return &result, nil
}

// Apply applies a collection of option-setting functions to the
// receiver, modifying it in-place.
func (opts *ConstructionOptions) Apply(options []ConstructionOption) error {
	for _, opt := range options {
		if err := opt(opts); err != nil {
			return err
		}
	}
	return nil
}

// ConstructionOption describes a named-parameter setting function for
// configuring a ConstructionOptions instance
type ConstructionOption func(*ConstructionOptions) error

// WithMirrorMode asks the constructor to create a Backend in "mirror mode". Not
// all backends will support this.
func WithMirrorMode(mirror bool) ConstructionOption {
	return func(opts *ConstructionOptions) error {
		opts.MirrorMode = mirror
		return nil
	}
}

// WithConcurrentBackend asks the constructor to create a
func WithConcurrentBackend(target backend.Backend) ConstructionOption {
	return func(opts *ConstructionOptions) error {
		opts.ConcurrentBackend = target
		return nil
	}
}

// BlockingFakeClock simulates a fake clock by
// sleeping instead of advancing an actual fake clock.
// This is required for backend clients which cannot
// time travel via a fake clock.
type BlockingFakeClock struct {
	clockwork.Clock
}

func (r BlockingFakeClock) Advance(d time.Duration) {
	if d < 0 {
		panic("Invalid argument, negative duration")
	}

	// We cannot rewind time for etcd since it will not have any effect on the server
	// so we actually sleep in this case
	time.Sleep(d)
}

func (r BlockingFakeClock) BlockUntil(int) {
	panic("Not implemented")
}

// Constructor describes a function for constructing new instances of a
// backend, with various options as required by a given test. Note that
// it's the caller's responsibility to close it when the test is finished.
type Constructor func(options ...ConstructionOption) (backend.Backend, clockwork.FakeClock, error)

// RunBackendComplianceSuite runs the entire backend compliance suite,
// creating a collection of named subtests under the context provided
// by `t`.
//
// As each test requires a new backend instance it will invoke the supplied
// `newBackend` function, which callers will use inject instances of the
// backend under test.
func RunBackendComplianceSuite(t *testing.T, newBackend Constructor) {
	t.Run("CRUD", func(t *testing.T) {
		testCRUD(t, newBackend)
	})

	t.Run("QueryRange", func(t *testing.T) {
		testQueryRange(t, newBackend)
	})

	t.Run("DeleteRange", func(t *testing.T) {
		testDeleteRange(t, newBackend)
	})

	t.Run("PutRange", func(t *testing.T) {
		testPutRange(t, newBackend)
	})

	t.Run("CompareAndSwap", func(t *testing.T) {
		testCompareAndSwap(t, newBackend)
	})

	t.Run("Expiration", func(t *testing.T) {
		testExpiration(t, newBackend)
	})

	t.Run("KeepAlive", func(t *testing.T) {
		testKeepAlive(t, newBackend)
	})

	t.Run("Events", func(t *testing.T) {
		testEvents(t, newBackend)
	})
	t.Run("WatchersClose", func(t *testing.T) {
		testWatchersClose(t, newBackend)
	})

	t.Run("Locking", func(t *testing.T) {
		testLocking(t, newBackend)
	})

	t.Run("ConcurrentOperations", func(t *testing.T) {
		testConcurrentOperations(t, newBackend)
	})

	t.Run("Mirror", func(t *testing.T) {
		testMirror(t, newBackend)
	})

	t.Run("FetchLimit", func(t *testing.T) {
		testFetchLimit(t, newBackend)
	})

	t.Run("Limit", func(t *testing.T) {
		testLimit(t, newBackend)
	})
}

// RequireItems asserts that the supplied `actual` items collection matches
// the `expected` collection, in size, ordering and the key/value pairs of
// each entry.
func RequireItems(t *testing.T, expected, actual []backend.Item) {
	require.Len(t, actual, len(expected))
	for i := range expected {
		require.Equal(t, expected[i].Key, actual[i].Key)
		require.Equal(t, expected[i].Value, actual[i].Value)
	}
}

// testCRUD tests create read update scenarios
func testCRUD(t *testing.T, newBackend Constructor) {
	uut, _, err := newBackend()
	require.NoError(t, err)
	defer func() { require.NoError(t, uut.Close()) }()

	ctx := context.Background()
	prefix := MakePrefix()

	item := backend.Item{Key: prefix("/hello"), Value: []byte("world")}

	// update will fail on non-existent item
	_, err = uut.Update(ctx, item)
	require.True(t, trace.IsNotFound(err))

	_, err = uut.Create(ctx, item)
	require.NoError(t, err)

	// create will fail on existing item
	_, err = uut.Create(context.Background(), item)
	require.True(t, trace.IsAlreadyExists(err))

	// get succeeds
	out, err := uut.Get(ctx, item.Key)
	require.NoError(t, err)
	require.Equal(t, item.Value, out.Value)

	// get range succeeds
	res, err := uut.GetRange(ctx, item.Key, backend.RangeEnd(item.Key), backend.NoLimit)
	require.NoError(t, err)
	require.Len(t, res.Items, 1)
	RequireItems(t, []backend.Item{item}, res.Items)

	// update succeeds
	updated := backend.Item{Key: prefix("/hello"), Value: []byte("world 2")}
	_, err = uut.Update(ctx, updated)
	require.NoError(t, err)

	out, err = uut.Get(ctx, item.Key)
	require.NoError(t, err)
	require.Equal(t, updated.Value, out.Value)

	// delete succeeds
	require.NoError(t, uut.Delete(ctx, item.Key))
	_, err = uut.Get(ctx, item.Key)
	require.True(t, trace.IsNotFound(err))

	// second delete won't find the item
	err = uut.Delete(ctx, item.Key)
	require.True(t, trace.IsNotFound(err))

	// put new item succeeds
	item = backend.Item{Key: prefix("/put"), Value: []byte("world")}
	_, err = uut.Put(ctx, item)
	require.NoError(t, err)

	out, err = uut.Get(ctx, item.Key)
	require.NoError(t, err)
	require.Equal(t, item.Value, out.Value)

	// put with large key and binary value succeeds.
	// NB: DynamoDB has a maximum overall key length of 1024 bytes, so
	//     we need to pick a random key size that will still fit in 1KiB
	//     when combined with the (currently) 33-byte prefix prepended
	//     by `prefix()`, so:
	//         (485 bytes * 2 (for hex encoding)) + 33 = 1003
	//     which gives us a little bit of room to spare
	keyBytes := make([]byte, 485)
	_, err = rand.Read(keyBytes)
	require.NoError(t, err)
	key := hex.EncodeToString(keyBytes)

	data := make([]byte, 1024)
	_, err = rand.Read(data)
	require.NoError(t, err)
	item = backend.Item{Key: prefix(key), Value: data}
	_, err = uut.Put(ctx, item)
	require.NoError(t, err)

	out, err = uut.Get(ctx, item.Key)
	require.NoError(t, err)
	require.Equal(t, item.Value, out.Value)
}

func testQueryRange(t *testing.T, newBackend Constructor) {
	uut, _, err := newBackend()
	require.NoError(t, err)
	defer func() { require.NoError(t, uut.Close()) }()

	ctx := context.Background()
	prefix := MakePrefix()

	outOfScope := backend.Item{Key: prefix("/a"), Value: []byte("should not show up")}
	a := backend.Item{Key: prefix("/prefix/a"), Value: []byte("val a")}
	b := backend.Item{Key: prefix("/prefix/b"), Value: []byte("val b")}
	c1 := backend.Item{Key: prefix("/prefix/c/c1"), Value: []byte("val c1")}
	c2 := backend.Item{Key: prefix("/prefix/c/c2"), Value: []byte("val c2")}

	for _, item := range []backend.Item{outOfScope, a, b, c1, c2} {
		_, err := uut.Create(ctx, item)
		require.NoError(t, err, "Failed creating value: %q => %q", item.Key, item.Value)
	}

	// prefix range fetch
	result, err := uut.GetRange(ctx, prefix("/prefix"), backend.RangeEnd(prefix("/prefix")), backend.NoLimit)
	require.NoError(t, err)
	RequireItems(t, []backend.Item{a, b, c1, c2}, result.Items)

	// sub prefix range fetch
	result, err = uut.GetRange(ctx, prefix("/prefix/c"), backend.RangeEnd(prefix("/prefix/c")), backend.NoLimit)
	require.NoError(t, err)
	RequireItems(t, []backend.Item{c1, c2}, result.Items)

	// range match
	result, err = uut.GetRange(ctx, prefix("/prefix/c/c1"), backend.RangeEnd(prefix("/prefix/c/cz")), backend.NoLimit)
	require.NoError(t, err)
	RequireItems(t, []backend.Item{c1, c2}, result.Items)

	// pagination
	result, err = uut.GetRange(ctx, prefix("/prefix"), backend.RangeEnd(prefix("/prefix")), 2)
	require.NoError(t, err)

	// expect two first records
	RequireItems(t, []backend.Item{a, b}, result.Items)

	// fetch next two items
	result, err = uut.GetRange(ctx, backend.RangeEnd(prefix("/prefix/b")), backend.RangeEnd(prefix("/prefix")), 2)
	require.NoError(t, err)

	// expect two last records
	RequireItems(t, []backend.Item{c1, c2}, result.Items)

	// next fetch is empty
	result, err = uut.GetRange(ctx, backend.RangeEnd(prefix("/prefix/c/c2")), backend.RangeEnd(prefix("/prefix")), 2)
	require.NoError(t, err)
	require.Empty(t, result.Items)
}

// testDeleteRange tests delete items by range
func testDeleteRange(t *testing.T, newBackend Constructor) {
	uut, _, err := newBackend()
	require.NoError(t, err)
	defer func() { require.NoError(t, uut.Close()) }()

	ctx := context.Background()
	prefix := MakePrefix()

	a := backend.Item{Key: prefix("/prefix/a"), Value: []byte("val a")}
	b := backend.Item{Key: prefix("/prefix/b"), Value: []byte("val b")}
	c1 := backend.Item{Key: prefix("/prefix/c/c1"), Value: []byte("val c1")}
	c2 := backend.Item{Key: prefix("/prefix/c/c2"), Value: []byte("val c2")}

	for _, item := range []backend.Item{a, b, c1, c2} {
		_, err := uut.Create(ctx, item)
		require.NoError(t, err, "Failed creating value: %q => %q", item.Key, item.Value)
	}

	err = uut.DeleteRange(ctx, prefix("/prefix/c"), backend.RangeEnd(prefix("/prefix/c")))
	require.NoError(t, err)

	// make sure items with "/prefix/c" are gone
	result, err := uut.GetRange(ctx, prefix("/prefix"), backend.RangeEnd(prefix("/prefix")), backend.NoLimit)
	require.NoError(t, err)
	RequireItems(t, []backend.Item{a, b}, result.Items)
}

// testPutRange tests scenarios with put range
func testPutRange(t *testing.T, newBackend Constructor) {
	uut, _, err := newBackend()
	require.NoError(t, err)
	defer func() { require.NoError(t, uut.Close()) }()

	batchUut, ok := uut.(backend.Batch)
	if !ok {
		t.Skip("Backend should support Batch interface for this test")
	}

	ctx := context.Background()
	prefix := MakePrefix()
	a := backend.Item{Key: prefix("/prefix/a"), Value: []byte("val a")}
	b := backend.Item{Key: prefix("/prefix/b"), Value: []byte("val b")}

	// add one element that should not show up (i.e. a duplicate `a`)
	err = batchUut.PutRange(ctx, []backend.Item{a, b, a})
	require.NoError(t, err)

	// prefix range fetch
	result, err := uut.GetRange(ctx, prefix("/prefix"), backend.RangeEnd(prefix("/prefix")), backend.NoLimit)
	require.NoError(t, err)
	RequireItems(t, []backend.Item{a, b}, result.Items)
}

// testCompareAndSwap tests compare and swap functionality
func testCompareAndSwap(t *testing.T, newBackend Constructor) {
	uut, _, err := newBackend()
	require.NoError(t, err)
	defer func() { require.NoError(t, uut.Close()) }()

	prefix := MakePrefix()
	ctx := context.Background()

	key := prefix("one")

	// compare and swap on non existing value will fail
	_, err = uut.CompareAndSwap(ctx, backend.Item{Key: key, Value: []byte("1")}, backend.Item{Key: key, Value: []byte("2")})
	require.True(t, trace.IsCompareFailed(err))

	// create value and try again...
	_, err = uut.Create(ctx, backend.Item{Key: key, Value: []byte("1")})
	require.NoError(t, err)

	// success CAS!
	_, err = uut.CompareAndSwap(ctx, backend.Item{Key: key, Value: []byte("1")}, backend.Item{Key: key, Value: []byte("2")})
	require.NoError(t, err)

	out, err := uut.Get(ctx, key)
	require.NoError(t, err)
	require.Equal(t, []byte("2"), out.Value)

	// value has been updated - not '1' any more
	_, err = uut.CompareAndSwap(ctx, backend.Item{Key: key, Value: []byte("1")}, backend.Item{Key: key, Value: []byte("3")})
	require.True(t, trace.IsCompareFailed(err))

	// existing value has not been changed by the failed CAS operation
	out, err = uut.Get(ctx, key)
	require.NoError(t, err)
	require.Equal(t, []byte("2"), out.Value)

	for i := 0; i < 10; i++ {
		i := i
		var wg sync.WaitGroup
		wg.Add(1)
		errs := make(chan error, 2)
		go func(value byte) {
			defer wg.Done()
			_, err := uut.CompareAndSwap(ctx, backend.Item{Key: key, Value: out.Value}, backend.Item{Key: key, Value: []byte{value}})
			errs <- err
		}(byte(i + 10))

		wg.Add(1)
		go func(value byte) {
			defer wg.Done()
			_, err := uut.CompareAndSwap(ctx, backend.Item{Key: key, Value: out.Value}, backend.Item{Key: key, Value: []byte{value}})
			errs <- err
		}(byte(i + 100))

		// validate that only a single failure occurred
		var failed int
		for i := 0; i < 2; i++ {
			err := <-errs
			if err != nil {
				failed++
			}
		}
		require.Equal(t, 1, failed)

		// validate that the value for the key was updated - we
		// don't care which CAS above won only that one of them
		// succeeded.
		item, err := uut.Get(ctx, key)
		require.NoError(t, err)
		require.NotEqual(t, out.Value, item.Value)
		out = item
	}
}

// testExpiration tests scenario with expiring values
func testExpiration(t *testing.T, newBackend Constructor) {
	uut, clock, err := newBackend()
	require.NoError(t, err)
	defer func() { require.NoError(t, uut.Close()) }()

	prefix := MakePrefix()
	ctx := context.Background()

	itemA := backend.Item{Key: prefix("a"), Value: []byte("val1")}
	_, err = uut.Put(ctx, itemA)
	require.NoError(t, err)

	itemB := backend.Item{Key: prefix("b"), Value: []byte("val1"), Expires: clock.Now().Add(1 * time.Second)}
	_, err = uut.Put(ctx, itemB)
	require.NoError(t, err)

	clock.Advance(4 * time.Second)

	res, err := uut.GetRange(ctx, prefix(""), backend.RangeEnd(prefix("")), backend.NoLimit)
	require.NoError(t, err)
	RequireItems(t, []backend.Item{itemA}, res.Items)
}

// addSeconds adds seconds with a seconds precision
// always rounding up to the next second,
// because TTL engines are usually 1 second precision
func addSeconds(t time.Time, seconds int64) time.Time {
	return time.Unix(t.UTC().Unix()+seconds+1, 0)
}

// testKeepAlive tests keep alive API
func testKeepAlive(t *testing.T, newBackend Constructor) {
	uut, clock, err := newBackend()
	require.NoError(t, err)
	defer func() { require.NoError(t, uut.Close()) }()

	prefix := MakePrefix()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// When I create a new watcher...
	watcher, err := uut.NewWatcher(ctx, backend.Watch{Prefixes: [][]byte{prefix("")}})
	require.NoError(t, err)
	defer func() { require.NoError(t, watcher.Close()) }()

	// ...expect that the event channel contains the original `init` message
	// sent when the Firestore client was set up.
	init := collectEvents(ctx, t, watcher, 1)
	requireEvents(t, init, []backend.Event{
		{Type: types.OpInit, Item: backend.Item{}},
	})

	// When I create an item that expires in 10 seconds and add it to the DB
	expiresAt := addSeconds(clock.Now(), 10)
	item, lease := AddItem(ctx, t, uut, prefix("key"), "val1", expiresAt)

	events := collectEvents(ctx, t, watcher, 1)
	requireEvents(t, events, []backend.Event{
		{Type: types.OpPut, Item: backend.Item{Key: prefix("key"), Value: []byte("val1"), Expires: expiresAt}},
	})

	// move the current slightly forward, but still *before* the item's
	// expiry time
	clock.Advance(2 * time.Second)

	// Move the item's expiration further in the future using a KeepAlive
	updatedAt := addSeconds(clock.Now(), 60)
	err = uut.KeepAlive(ctx, lease, updatedAt)
	require.NoError(t, err)

	// Since the backend translates absolute expiration timestamp to a TTL
	// and collecting events takes arbitrary time, the expiration timestamps
	// on the collected events might have a slight skew
	events = collectEvents(ctx, t, watcher, 1)
	requireEvents(t, events, []backend.Event{
		{Type: types.OpPut, Item: backend.Item{Key: prefix("key"), Value: []byte("val1"), Expires: updatedAt}},
	})

	err = uut.Delete(context.TODO(), item.Key)
	require.NoError(t, err)

	_, err = uut.Get(context.TODO(), item.Key)
	require.True(t, trace.IsNotFound(err))

	// keep alive on deleted or expired object should fail
	err = uut.KeepAlive(context.TODO(), lease, updatedAt.Add(1*time.Second))
	require.True(t, trace.IsNotFound(err))
}

func collectEvents(ctx context.Context, t *testing.T, watcher backend.Watcher, count int) []backend.Event {
	var events []backend.Event
	for i := 0; i < count; i++ {
		select {
		case e := <-watcher.Events():
			events = append(events, e)
		case <-watcher.Done():
			require.FailNow(t, "Watcher has unexpectedly closed.")
		case <-ctx.Done():
			require.FailNowf(t, "Context expired waiting for events.",
				"Captured %d of %d so far: %v", len(events), count, events)
		}
	}
	return events
}

// testEvents tests scenarios with event watches
func testEvents(t *testing.T, newBackend Constructor) {
	eventTimeout := 10 * time.Second

	uut, clock, err := newBackend()
	require.NoError(t, err)
	defer func() { require.NoError(t, uut.Close()) }()

	prefix := MakePrefix()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a new watcher for the test prefix.
	watcher, err := uut.NewWatcher(ctx, backend.Watch{Prefixes: [][]byte{prefix("")}})
	require.NoError(t, err)
	defer func() { require.NoError(t, watcher.Close()) }()

	// Make sure INIT event is emitted.
	requireEvent(t, watcher, types.OpInit, nil, eventTimeout)

	// Add item to backend.
	item := &backend.Item{Key: prefix("b"), Value: []byte("val")}
	_, err = uut.Put(ctx, *item)
	require.NoError(t, err)

	// Make sure item was added into backend.
	item, err = uut.Get(ctx, item.Key)
	require.NoError(t, err)

	// Make sure a PUT event is emitted.
	e := requireEvent(t, watcher, types.OpPut, item.Key, eventTimeout)
	require.Equal(t, item.Value, e.Item.Value)

	// Delete item from backend.
	err = uut.Delete(ctx, item.Key)
	require.NoError(t, err)

	// Make sure item is no longer in backend.
	_, err = uut.Get(ctx, item.Key)
	require.True(t, trace.IsNotFound(err), "Item should have been be deleted")

	// Make sure a DELETE event is emitted.
	requireEvent(t, watcher, types.OpDelete, item.Key, eventTimeout)

	// Add item to backend with a 1 second TTL.
	item = &backend.Item{
		Key:     prefix("c"),
		Value:   []byte("val"),
		Expires: clock.Now().Add(1 * time.Second),
	}
	_, err = uut.Put(ctx, *item)
	require.NoError(t, err)

	// Make sure item was added into backend.
	item, err = uut.Get(ctx, item.Key)
	require.NoError(t, err)

	// Make sure a PUT event is emitted.
	e = requireEvent(t, watcher, types.OpPut, item.Key, eventTimeout)
	require.Equal(t, item.Value, e.Item.Value)

	// Wait a few seconds for the item to expire.
	clock.Advance(3 * time.Second)

	// Make sure item has been removed.
	_, err = uut.Get(ctx, item.Key)
	require.Error(t, err)

	// Make sure a DELETE event is emitted.
	requireEvent(t, watcher, types.OpDelete, item.Key, 2*time.Second)
}

// testFetchLimit tests fetch max items size limit.
func testFetchLimit(t *testing.T, newBackend Constructor) {
	uut, _, err := newBackend()
	require.NoError(t, err)
	defer func() { require.NoError(t, uut.Close()) }()

	prefix := MakePrefix()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Allocate 65KB buffer.
	buff := make([]byte, 1<<16)
	itemsCount := 20
	// Fill the backend with events that total size is greater than 1MB (65KB * 20 > 1MB).
	for i := 0; i < itemsCount; i++ {
		item := &backend.Item{Key: prefix(fmt.Sprintf("/db/database%d", i)), Value: buff}
		_, err = uut.Put(ctx, *item)
		require.NoError(t, err)
	}

	result, err := uut.GetRange(ctx, prefix("/db"), backend.RangeEnd(prefix("/db")), backend.NoLimit)
	require.NoError(t, err)
	require.Equal(t, itemsCount, len(result.Items))
}

// testLimit tests limit.
func testLimit(t *testing.T, newBackend Constructor) {
	uut, clock, err := newBackend()
	require.NoError(t, err)
	defer func() { require.NoError(t, uut.Close()) }()

	prefix := MakePrefix()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	item := &backend.Item{
		Key:     prefix("/db/database_tail_item"),
		Value:   []byte("data"),
		Expires: clock.Now().Add(time.Minute),
	}
	_, err = uut.Put(ctx, *item)
	require.NoError(t, err)
	for i := 0; i < 10; i++ {
		item := &backend.Item{
			Key:     prefix(fmt.Sprintf("/db/database%d", i)),
			Value:   []byte("data"),
			Expires: clock.Now().Add(time.Second * 10),
		}
		_, err = uut.Put(ctx, *item)
		require.NoError(t, err)
	}
	clock.Advance(time.Second * 5)

	item = &backend.Item{
		Key:     prefix("/db/database_head_item"),
		Value:   []byte("data"),
		Expires: clock.Now().Add(time.Minute),
	}
	_, err = uut.Put(ctx, *item)
	require.NoError(t, err)

	result, err := uut.GetRange(ctx, prefix("/db"), backend.RangeEnd(prefix("/db")), 2)
	require.NoError(t, err)
	require.Equal(t, 2, len(result.Items))
}

// requireEvent asserts that a given event type with the given key is emitted
// by a watcher within the supplied timeout, returning that event for further
// inspection if successful.
func requireEvent(t *testing.T, watcher backend.Watcher, eventType types.OpType, key []byte, timeout time.Duration) backend.Event {
	t.Helper()
	select {
	case e := <-watcher.Events():
		require.Equal(t, eventType, e.Type)
		require.Equal(t, key, e.Item.Key)
		return e

	case <-watcher.Done():
		require.FailNow(t, "Watcher has unexpectedly closed.")

	case <-time.After(timeout):
		require.FailNowf(t, "Timed out", "Timed out after %v waiting for event %v", timeout.String(), eventType.String())
	}

	return backend.Event{}
}

// requireNoEvent asserts that no events of any kind are emitted by the given
// watcher in the supplied timeframe.
func requireNoEvent(t *testing.T, watcher backend.Watcher, timeout time.Duration) {
	select {
	case e := <-watcher.Events():
		require.FailNowf(t, "Unexpected event", "%s %q => %q", e.Type, e.Item.Key, e.Item.Value)

	case <-watcher.Done():
		require.FailNow(t, "Watcher has unexpectedly closed.")

	case <-time.After(timeout):
		return // Success!
	}
}

// WatchersClose tests scenarios with watches close
func testWatchersClose(t *testing.T, newBackend Constructor) {
	uut, _, err := newBackend()
	require.NoError(t, err)

	// The test function explicitly closes the UUT backend, so we only
	// want this deferred call for emergency cleanup, rather than part
	// of the tests itself. This is why we're not checking the error
	// here as it will almost always fail with something like "already
	// closed"
	defer uut.Close()

	prefix := MakePrefix()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watcher, err := uut.NewWatcher(ctx, backend.Watch{Prefixes: [][]byte{prefix("")}})
	require.NoError(t, err)

	// cancel context -> get watcher to close
	cancel()

	select {
	case <-watcher.Done():
	case <-time.After(time.Second):
		require.FailNow(t, "Timeout waiting for watcher to close")
	}

	// closing backend should close associated watcher too
	watcher, err = uut.NewWatcher(context.Background(), backend.Watch{Prefixes: [][]byte{prefix("")}})
	require.NoError(t, err)

	require.NoError(t, uut.Close())

	select {
	case <-watcher.Done():
	case <-time.After(time.Second):
		require.FailNow(t, "Timeout waiting for watcher to close")
	}
}

// testLocking tests locking logic
func testLocking(t *testing.T, newBackend Constructor) {
	tok1 := "token1"
	tok2 := "token2"
	ttl := 5 * time.Second

	uut, clock, err := newBackend()
	require.NoError(t, err)
	defer func() { require.NoError(t, uut.Close()) }()

	// If all this takes more than a minute then something external to the test
	// has probably gone bad (e.g. db server has ceased to exist), so it's
	// probably best to bail out with a sensible error (& call stack) rather
	// than wait for the test to time out
	ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Minute)
	defer cancel()

	// Manually drive the clock at ~10x speed to make sure anyone waiting on it
	// will eventually be woken. This will automatically be stopped when the
	// test exits thanks to the deferred cancel above.
	go func() {
		t := time.NewTicker(100 * time.Millisecond)
		defer t.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-t.C:
				clock.Advance(1 * time.Second)
			}
		}
	}()

	// some bookkeeping to make sure that any errors that happen asynchronously
	// will be tracked and bubble back to fail this test. Note that this will
	// also ensure that the `uut` Backend will remain alive until all async
	// operations have completed.
	asyncOps := sync.WaitGroup{}
	asyncErrs := make(chan error, 10)
	requireNoAsyncErrors := func() {
		requireWaitGroupToFinish(ctx, t, &asyncOps)
		select {
		case err := <-asyncErrs:
			require.NoError(t, err)
		default:
			// Happy path - there were no async errors!
		}
	}
	defer requireNoAsyncErrors()

	// Given a lock named `tok1` on the backend...
	lock, err := backend.AcquireLock(ctx, uut, tok1, ttl)
	require.NoError(t, err)

	//  When I asynchronously release the lock...
	marker := int32(7)
	asyncOps.Add(1)
	go func() {
		defer asyncOps.Done()
		atomic.StoreInt32(&marker, 9)
		if err := lock.Release(ctx, uut); err != nil {
			asyncErrs <- err
		}
	}()

	// ...and simultaneously attempt to create a new lock with the same name
	lock, err = backend.AcquireLock(ctx, uut, tok1, ttl)

	// expect that the asynchronous Release() has executed - we're using the
	// change in the value of the marker value as a proxy for the Release().
	atomic.AddInt32(&marker, 9)
	require.Equal(t, int32(18), atomic.LoadInt32(&marker))

	// ...and also expect that the acquire succeeded, and will release safely.
	require.NoError(t, err)
	require.NoError(t, lock.Release(ctx, uut))

	// Given a lock with the same name as previously-existing, manually-released lock
	lock, err = backend.AcquireLock(ctx, uut, tok1, ttl)
	require.NoError(t, err)
	atomic.StoreInt32(&marker, 7)

	//  When I asynchronously release the lock...
	asyncOps.Add(1)
	go func() {
		defer asyncOps.Done()
		atomic.StoreInt32(&marker, 9)
		if err := lock.Release(ctx, uut); err != nil {
			asyncErrs <- err
		}
	}()

	// ...and simultaneously try to acquire another lock with the same name
	lock, err = backend.AcquireLock(ctx, uut, tok1, ttl)

	// expect that the asynchronous Release() has executed - we're using the
	// change in the value of the marker value as a proxy for the call to
	// Release().
	atomic.AddInt32(&marker, 9)
	require.Equal(t, int32(18), atomic.LoadInt32(&marker))

	// ...and also expect that the acquire succeeded, and will release safely.
	require.NoError(t, err)
	require.NoError(t, lock.Release(ctx, uut))

	// Given a pair of locks named `tok1` and `tok2`
	y := int32(0)
	lock1, err := backend.AcquireLock(ctx, uut, tok1, ttl)
	require.NoError(t, err)
	lock2, err := backend.AcquireLock(ctx, uut, tok2, ttl)
	require.NoError(t, err)

	//  When I asynchronously release the locks...
	asyncOps.Add(1)
	go func() {
		defer asyncOps.Done()
		atomic.StoreInt32(&y, 15)
		if err := lock1.Release(ctx, uut); err != nil {
			asyncErrs <- err
		}

		if err := lock2.Release(ctx, uut); err != nil {
			asyncErrs <- err
		}
	}()

	lock, err = backend.AcquireLock(ctx, uut, tok1, ttl)
	require.NoError(t, err)
	require.Equal(t, int32(15), atomic.LoadInt32(&y))
	require.NoError(t, lock.Release(ctx, uut))
}

// testConcurrentOperations tests concurrent operations on the same
// shared backend
func testConcurrentOperations(t *testing.T, newBackend Constructor) {
	uutA, _, err := newBackend()
	require.NoError(t, err)
	defer func() { require.NoError(t, uutA.Close()) }()

	uutB, _, err := newBackend(WithConcurrentBackend(uutA))
	if err == ErrConcurrentAccessNotSupported {
		t.Skip("Backend does not support concurrent access")
	}
	require.NoError(t, err)
	defer func() { require.NoError(t, uutB.Close()) }()

	prefix := MakePrefix()
	ctx := context.TODO()
	value1 := "this first value should not be corrupted by concurrent ops"
	value2 := "this second value should not be corrupted too"
	const attempts = 50

	asyncOps := sync.WaitGroup{}
	asyncErrs := make(chan error, 5*attempts)

	for i := 0; i < attempts; i++ {
		asyncOps.Add(5)

		go func(cnt int) {
			defer asyncOps.Done()
			_, err := uutA.Put(ctx, backend.Item{Key: prefix("key"), Value: []byte(value1)})
			if err != nil {
				asyncErrs <- err
			}
		}(i)

		go func(cnt int) {
			defer asyncOps.Done()
			_, err := uutB.CompareAndSwap(ctx,
				backend.Item{Key: prefix("key"), Value: []byte(value2)},
				backend.Item{Key: prefix("key"), Value: []byte(value1)})
			if err != nil && !trace.IsCompareFailed(err) {
				asyncErrs <- err
			}
		}(i)

		go func(cnt int) {
			defer asyncOps.Done()
			_, err := uutB.Create(ctx, backend.Item{Key: prefix("key"), Value: []byte(value2)})
			if err != nil && !trace.IsAlreadyExists(err) {
				asyncErrs <- err
			}
		}(i)

		go func(cnt int) {
			defer asyncOps.Done()
			item, err := uutA.Get(ctx, prefix("key"))
			if err != nil && !trace.IsNotFound(err) {
				asyncErrs <- err
			}

			// make sure data is not corrupted along the way
			if err == nil {
				val := string(item.Value)
				if val != value1 && val != value2 {
					asyncErrs <- trace.Errorf(
						"corruption detected. expected one of %q or %q and got %q", value1, value2, val)
				}
			}
		}(i)

		go func(cnt int) {
			defer asyncOps.Done()
			err := uutB.Delete(ctx, prefix("key"))
			if err != nil && !trace.IsNotFound(err) {
				t.Logf("Error %v", err)
				asyncErrs <- err
			}
		}(i)
	}

	// Give the database some time to update. A single-node in-memory database
	// will finish faster than a 3-node cluster. Some latency is expected
	// since this test intentionally creates conflict on the same key. Most tests
	// should complete in less than a few seconds.
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 10*time.Second)
	defer timeoutCancel()
	requireWaitGroupToFinish(timeoutCtx, t, &asyncOps)

	select {
	case e := <-asyncErrs:
		require.NoError(t, e)
	default:
		// Happy path - no async errors occurred
	}
}

// Mirror tests mirror mode for backends (used in caches). Only some backends
// support mirror mode (like memory).
func testMirror(t *testing.T, newBackend Constructor) {
	uut, _, err := newBackend(WithMirrorMode(true))
	if err == ErrMirrorNotSupported {
		t.Skip("Backend does not support mirror mode")
	}
	require.NoError(t, err)
	defer func() { require.NoError(t, uut.Close()) }()

	prefix := MakePrefix()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a new watcher for the test prefix.
	watcher, err := uut.NewWatcher(ctx, backend.Watch{Prefixes: [][]byte{prefix("")}})
	require.NoError(t, err)
	defer func() { require.NoError(t, watcher.Close()) }()

	// Make sure INIT event is emitted.
	requireEvent(t, watcher, types.OpInit, nil, 2*time.Second)

	// Add item to backend with a 1 second TTL.
	item := &backend.Item{
		Key:     prefix("a"),
		Value:   []byte("val"),
		Expires: uut.Clock().Now().Add(1 * time.Second),
	}
	_, err = uut.Put(ctx, *item)
	require.NoError(t, err)

	// Make sure item was added into backend.
	item, err = uut.Get(ctx, item.Key)
	require.NoError(t, err)

	// Save the original ID, later in this test after an update, the ID should
	// not have changed in mirror mode.
	originalID := item.ID

	// Make sure a PUT event is emitted.
	e := requireEvent(t, watcher, types.OpPut, item.Key, 2*time.Second)
	require.Equal(t, item.Value, e.Item.Value)

	// Wait 1 second for the item to expire.
	time.Sleep(time.Second)

	// Make sure item has not been removed.
	nitem, err := uut.Get(ctx, item.Key)
	require.NoError(t, err)
	require.Equal(t, item.Key, nitem.Key)
	require.Equal(t, item.Value, nitem.Value)

	// Make sure a DELETE event was not emitted.
	requireNoEvent(t, watcher, 2*time.Second)

	// Update the existing item.
	_, err = uut.Put(ctx, backend.Item{
		Key:   prefix("a"),
		Value: []byte("val2"),
	})
	require.NoError(t, err)

	// Get update item and make sure that the ID has not changed.
	item, err = uut.Get(ctx, prefix("a"))
	require.NoError(t, err)
	require.Equal(t, originalID, item.ID)

	// Add item to backend that is already expired.
	item2 := &backend.Item{
		Key:     prefix("b"),
		Value:   []byte("val"),
		Expires: uut.Clock().Now().Add(-1 * time.Second),
	}
	_, err = uut.Put(ctx, *item2)
	require.NoError(t, err)

	// Make sure item was added into backend despite being expired.
	_, err = uut.Get(ctx, item2.Key)
	require.NoError(t, err)
}

func AddItem(ctx context.Context, t *testing.T, uut backend.Backend, key []byte, value string, expires time.Time) (backend.Item, backend.Lease) {
	t.Helper()
	item := backend.Item{
		Key:     key,
		Value:   []byte(value),
		Expires: expires,
	}
	lease, err := uut.Put(ctx, item)
	require.NoError(t, err)
	return item, *lease
}

// requireWaitGroupToFinish asserts that the given WaitGroup must finish all of
// its outstanding tasks before the supplied context expires.
func requireWaitGroupToFinish(ctx context.Context, t *testing.T, waitGroup *sync.WaitGroup) {
	wgDone := make(chan struct{})
	go func() {
		defer close(wgDone)
		waitGroup.Wait()
	}()
	select {
	case <-wgDone:
		return

	case <-ctx.Done():
		require.FailNowf(t, "Context expired waiting for WaitGroup", "context: %s", ctx.Err())
	}
}

// MakePrefix returns function that appends unique prefix
// to any key, used to make test suite concurrent-run proof
func MakePrefix() func(k string) []byte {
	id := "/" + uuid.New().String()
	return func(k string) []byte {
		return []byte(id + k)
	}
}

func requireEvents(t *testing.T, obtained, expected []backend.Event) {
	requireIncreasingIDs(t, obtained)
	requireNoDuplicateIDs(t, obtained)
	requireExpireTimestamps(t, obtained, expected)
}

func requireIncreasingIDs(t *testing.T, obtained []backend.Event) {
	lastID := int64(-1)
	for _, item := range obtained {
		require.Greater(t, item.Item.ID, lastID)
		lastID = item.Item.ID
	}
}

func requireNoDuplicateIDs(t *testing.T, obtained []backend.Event) {
	set := make(map[int64]struct{})
	for _, event := range obtained {
		_, ok := set[event.Item.ID]
		require.False(t, ok, "Duplicate ID for %v.", event.Item.ID)
		set[event.Item.ID] = struct{}{}
	}
}

// requireExpireTimestampsIncreasing verifies that the expiry timestamps
// of the `obtained` items expire _after_ the corresponding `expected`
// item expiry times
func requireExpireTimestamps(t *testing.T, obtained, expected []backend.Event) {
	require.Len(t, obtained, len(expected))

	for i := range expected {
		require.False(t,
			obtained[i].Item.Expires.After(expected[i].Item.Expires),
			"Expected %v >= %v",
			expected[i].Item.Expires,
			obtained[i].Item.Expires,
		)
	}
}
