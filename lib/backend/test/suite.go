/*
Copyright 2015-2018 Gravitational, Inc.

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
	"encoding/hex"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/fixtures"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"gopkg.in/check.v1"
)

type BackendSuite struct {
	B backend.Backend
	// B2 is a backend opened to the same database,
	// used for concurrent operations tests
	B2         backend.Backend
	NewBackend func() (backend.Backend, error)
	Clock      clockwork.FakeClock
}

// CRUD tests create read update scenarios
func (s *BackendSuite) CRUD(c *check.C) {
	ctx := context.Background()

	prefix := MakePrefix()

	item := backend.Item{Key: prefix("/hello"), Value: []byte("world")}

	// update will fail on non-existent item
	_, err := s.B.Update(ctx, item)
	fixtures.ExpectNotFound(c, err)

	_, err = s.B.Create(ctx, item)
	c.Assert(err, check.IsNil)

	// create will fail on existing item
	_, err = s.B.Create(context.Background(), item)
	fixtures.ExpectAlreadyExists(c, err)

	// get succeeds
	out, err := s.B.Get(ctx, item.Key)
	c.Assert(err, check.IsNil)
	c.Assert(string(out.Value), check.Equals, string(item.Value))

	// get range succeeds
	res, err := s.B.GetRange(ctx, item.Key, backend.RangeEnd(item.Key), backend.NoLimit)
	c.Assert(err, check.IsNil)
	c.Assert(len(res.Items), check.Equals, 1)
	c.Assert(string(res.Items[0].Value), check.Equals, string(item.Value))
	c.Assert(string(res.Items[0].Key), check.Equals, string(item.Key))

	// update succeeds
	updated := backend.Item{Key: prefix("/hello"), Value: []byte("world 2")}
	_, err = s.B.Update(ctx, updated)
	c.Assert(err, check.IsNil)

	out, err = s.B.Get(ctx, item.Key)
	c.Assert(err, check.IsNil)
	c.Assert(string(out.Value), check.Equals, string(updated.Value))

	// delete succeeds
	err = s.B.Delete(ctx, item.Key)
	c.Assert(err, check.IsNil)

	_, err = s.B.Get(ctx, item.Key)
	fixtures.ExpectNotFound(c, err)

	// second delete won't find the item
	err = s.B.Delete(ctx, item.Key)
	fixtures.ExpectNotFound(c, err)

	// put new item succeeds
	item = backend.Item{Key: prefix("/put"), Value: []byte("world")}
	_, err = s.B.Put(ctx, item)
	c.Assert(err, check.IsNil)

	out, err = s.B.Get(ctx, item.Key)
	c.Assert(err, check.IsNil)
	c.Assert(string(out.Value), check.Equals, string(item.Value))

	// put with large key and binary value succeeds.
	// NB: DynamoDB has a maximum overall key length of 1024 bytes, so
	//     we need to pick a random key size that will still fit in 1KiB
	//     when combined with the (currently) 33-byte prefix prepended
	//     by `prefix()`, so:
	//         (485 bytes * 2 (for hex encoding)) + 33 = 1003
	//     which gives us a little bit of room to spare
	keyBytes := make([]byte, 485)
	rand.Read(keyBytes)
	key := hex.EncodeToString(keyBytes)

	data := make([]byte, 1024)
	rand.Read(data)
	item = backend.Item{Key: prefix(key), Value: data}
	_, err = s.B.Put(ctx, item)
	c.Assert(err, check.IsNil)

	out, err = s.B.Get(ctx, item.Key)
	c.Assert(err, check.IsNil)
	c.Assert(out.Value, check.DeepEquals, item.Value)
}

// Range tests scenarios with range queries
func (s *BackendSuite) Range(c *check.C) {
	ctx := context.Background()
	prefix := MakePrefix()

	// add one element that should not show up
	_, err := s.B.Create(ctx, backend.Item{Key: prefix("/a"), Value: []byte("should not show up")})
	c.Assert(err, check.IsNil)

	_, err = s.B.Create(ctx, backend.Item{Key: prefix("/prefix/a"), Value: []byte("val a")})
	c.Assert(err, check.IsNil)

	_, err = s.B.Create(ctx, backend.Item{Key: prefix("/prefix/b"), Value: []byte("val b")})
	c.Assert(err, check.IsNil)

	_, err = s.B.Create(ctx, backend.Item{Key: prefix("/prefix/c/c1"), Value: []byte("val c1")})
	c.Assert(err, check.IsNil)

	_, err = s.B.Create(ctx, backend.Item{Key: prefix("/prefix/c/c2"), Value: []byte("val c2")})
	c.Assert(err, check.IsNil)

	// add element that does not match the range to make
	// sure it won't get included in the list
	_, err = s.B.Create(ctx, backend.Item{Key: prefix("a"), Value: []byte("no match a")})
	c.Assert(err, check.IsNil)

	// prefix range fetch
	result, err := s.B.GetRange(ctx, prefix("/prefix"), backend.RangeEnd(prefix("/prefix")), backend.NoLimit)
	c.Assert(err, check.IsNil)
	expected := []backend.Item{
		{Key: prefix("/prefix/a"), Value: []byte("val a")},
		{Key: prefix("/prefix/b"), Value: []byte("val b")},
		{Key: prefix("/prefix/c/c1"), Value: []byte("val c1")},
		{Key: prefix("/prefix/c/c2"), Value: []byte("val c2")},
	}
	ExpectItems(c, result.Items, expected)

	// sub prefix range fetch
	result, err = s.B.GetRange(ctx, prefix("/prefix/c"), backend.RangeEnd(prefix("/prefix/c")), backend.NoLimit)
	c.Assert(err, check.IsNil)
	expected = []backend.Item{
		{Key: prefix("/prefix/c/c1"), Value: []byte("val c1")},
		{Key: prefix("/prefix/c/c2"), Value: []byte("val c2")},
	}
	ExpectItems(c, result.Items, expected)

	// range match
	result, err = s.B.GetRange(ctx, prefix("/prefix/c/c1"), backend.RangeEnd(prefix("/prefix/c/cz")), backend.NoLimit)
	c.Assert(err, check.IsNil)
	ExpectItems(c, result.Items, expected)

	// pagination
	result, err = s.B.GetRange(ctx, prefix("/prefix"), backend.RangeEnd(prefix("/prefix")), 2)
	c.Assert(err, check.IsNil)
	// expect two first records
	expected = []backend.Item{
		{Key: prefix("/prefix/a"), Value: []byte("val a")},
		{Key: prefix("/prefix/b"), Value: []byte("val b")},
	}
	ExpectItems(c, result.Items, expected)

	// fetch next two items
	result, err = s.B.GetRange(ctx, backend.RangeEnd(prefix("/prefix/b")), backend.RangeEnd(prefix("/prefix")), 2)
	c.Assert(err, check.IsNil)

	// expect two last records
	expected = []backend.Item{
		{Key: prefix("/prefix/c/c1"), Value: []byte("val c1")},
		{Key: prefix("/prefix/c/c2"), Value: []byte("val c2")},
	}
	ExpectItems(c, result.Items, expected)

	// next fetch is empty
	result, err = s.B.GetRange(ctx, backend.RangeEnd(prefix("/prefix/c/c2")), backend.RangeEnd(prefix("/prefix")), 2)
	c.Assert(err, check.IsNil)
	c.Assert(result.Items, check.HasLen, 0)
}

// DeleteRange tests delete items by range
func (s *BackendSuite) DeleteRange(c *check.C) {
	ctx := context.Background()
	prefix := MakePrefix()

	_, err := s.B.Create(ctx, backend.Item{Key: prefix("/prefix/a"), Value: []byte("val a")})
	c.Assert(err, check.IsNil)

	_, err = s.B.Create(ctx, backend.Item{Key: prefix("/prefix/b"), Value: []byte("val b")})
	c.Assert(err, check.IsNil)

	_, err = s.B.Create(ctx, backend.Item{Key: prefix("/prefix/c/c1"), Value: []byte("val c1")})
	c.Assert(err, check.IsNil)

	_, err = s.B.Create(ctx, backend.Item{Key: prefix("/prefix/c/c2"), Value: []byte("val c2")})
	c.Assert(err, check.IsNil)

	err = s.B.DeleteRange(ctx, prefix("/prefix/c"), backend.RangeEnd(prefix("/prefix/c")))
	c.Assert(err, check.IsNil)

	// make sure items with "/prefix/c" are gone
	result, err := s.B.GetRange(ctx, prefix("/prefix"), backend.RangeEnd(prefix("/prefix")), backend.NoLimit)
	c.Assert(err, check.IsNil)
	expected := []backend.Item{
		{Key: prefix("/prefix/a"), Value: []byte("val a")},
		{Key: prefix("/prefix/b"), Value: []byte("val b")},
	}
	ExpectItems(c, result.Items, expected)
}

// PutRange tests scenarios with put range
func (s *BackendSuite) PutRange(c *check.C) {
	ctx := context.Background()
	prefix := MakePrefix()

	b, ok := s.B.(backend.Batch)
	if !ok {
		c.Fatalf("Backend should support Batch interface for this test")
	}

	// add one element that should not show up
	items := []backend.Item{
		{Key: prefix("/prefix/a"), Value: []byte("val a")},
		{Key: prefix("/prefix/b"), Value: []byte("val b")},
		{Key: prefix("/prefix/a"), Value: []byte("val a")},
	}
	err := b.PutRange(ctx, items)
	c.Assert(err, check.IsNil)

	// prefix range fetch
	result, err := s.B.GetRange(ctx, prefix("/prefix"), backend.RangeEnd(prefix("/prefix")), backend.NoLimit)
	c.Assert(err, check.IsNil)
	expected := []backend.Item{
		{Key: prefix("/prefix/a"), Value: []byte("val a")},
		{Key: prefix("/prefix/b"), Value: []byte("val b")},
	}
	ExpectItems(c, result.Items, expected)
}

// CompareAndSwap tests compare and swap functionality
func (s *BackendSuite) CompareAndSwap(c *check.C) {
	prefix := MakePrefix()
	ctx := context.Background()

	// compare and swap on non existing value will fail
	_, err := s.B.CompareAndSwap(ctx, backend.Item{Key: prefix("one"), Value: []byte("1")}, backend.Item{Key: prefix("one"), Value: []byte("2")})
	fixtures.ExpectCompareFailed(c, err)

	_, err = s.B.Create(ctx, backend.Item{Key: prefix("one"), Value: []byte("1")})
	c.Assert(err, check.IsNil)

	// success CAS!
	_, err = s.B.CompareAndSwap(ctx, backend.Item{Key: prefix("one"), Value: []byte("1")}, backend.Item{Key: prefix("one"), Value: []byte("2")})
	c.Assert(err, check.IsNil)

	out, err := s.B.Get(ctx, prefix("one"))
	c.Assert(err, check.IsNil)
	c.Assert(string(out.Value), check.Equals, "2")

	// value has been updated - not '1' any more
	_, err = s.B.CompareAndSwap(ctx, backend.Item{Key: prefix("one"), Value: []byte("1")}, backend.Item{Key: prefix("one"), Value: []byte("3")})
	fixtures.ExpectCompareFailed(c, err)

	// existing value has not been changed by the failed CAS operation
	out, err = s.B.Get(ctx, prefix("one"))
	c.Assert(err, check.IsNil)
	c.Assert(string(out.Value), check.Equals, "2")
}

// Expiration tests scenario with expiring values
func (s *BackendSuite) Expiration(c *check.C) {
	prefix := MakePrefix()
	ctx := context.Background()

	itemA := backend.Item{Key: prefix("a"), Value: []byte("val1")}
	_, err := s.B.Put(ctx, itemA)
	c.Assert(err, check.IsNil)

	_, err = s.B.Put(ctx, backend.Item{Key: prefix("b"), Value: []byte("val1"), Expires: s.Clock.Now().Add(1 * time.Second)})
	c.Assert(err, check.IsNil)

	s.Clock.Advance(4 * time.Second)
	res, err := s.B.GetRange(ctx, prefix(""), backend.RangeEnd(prefix("")), backend.NoLimit)
	c.Assert(err, check.IsNil)
	ExpectItems(c, res.Items, []backend.Item{itemA})
}

// addSeconds adds seconds with a seconds precision
// always rounding up to the next second,
// because TTL engines are usually 1 second precision
func addSeconds(t time.Time, seconds int64) time.Time {
	return time.Unix(t.UTC().Unix()+seconds+1, 0)
}

// KeepAlive tests keep alive API
func (s *BackendSuite) KeepAlive(c *check.C) {
	prefix := MakePrefix()
	ctx := context.Background()

	watcher, err := s.B.NewWatcher(ctx, backend.Watch{Prefixes: [][]byte{prefix("")}})
	c.Assert(err, check.IsNil)
	defer watcher.Close()

	init := collectEvents(c, watcher, 1)
	verifyEvents(c, init, []backend.Event{
		{Type: types.OpInit, Item: backend.Item{}},
	})

	expiresAt := addSeconds(s.Clock.Now(), 2)
	item, lease := s.addItem(context.TODO(), c, prefix("key"), "val1", expiresAt)

	s.Clock.Advance(1 * time.Second)

	// Move the expiration further in the future to avoid processing
	// skew and ensure the item is available when we delete it.
	// It does not affect the running time of the test
	updatedAt := addSeconds(s.Clock.Now(), 60)
	err = s.B.KeepAlive(context.TODO(), lease, updatedAt)
	c.Assert(err, check.IsNil)

	// Since the backend translates absolute expiration timestamp to a TTL
	// and collecting events takes arbitrary time, the expiration timestamps
	// on the collected events might have a slight skew
	events := collectEvents(c, watcher, 2)
	verifyEvents(c, events, []backend.Event{
		{Type: types.OpPut, Item: backend.Item{Key: prefix("key"), Value: []byte("val1"), Expires: expiresAt}},
		{Type: types.OpPut, Item: backend.Item{Key: prefix("key"), Value: []byte("val1"), Expires: updatedAt}},
	})

	err = s.B.Delete(context.TODO(), item.Key)
	require.NoError(c, err)
	c.Assert(err, check.IsNil)

	_, err = s.B.Get(context.TODO(), item.Key)
	c.Assert(err, check.FitsTypeOf, trace.NotFound(""))

	// keep alive on deleted or expired object should fail
	err = s.B.KeepAlive(context.TODO(), lease, updatedAt.Add(1*time.Second))
	c.Assert(err, check.FitsTypeOf, trace.NotFound(""))
}

func collectEvents(c *check.C, watcher backend.Watcher, count int) []backend.Event {
	var events []backend.Event
	for i := 0; i < count; i++ {
		select {
		case e := <-watcher.Events():
			events = append(events, e)
		case <-watcher.Done():
			c.Fatalf("Watcher has unexpectedly closed.")
		case <-time.After(2 * time.Second):
			c.Fatalf("Timeout waiting for event.")
		}
	}
	return events
}

// Events tests scenarios with event watches
func (s *BackendSuite) Events(c *check.C) {
	prefix := MakePrefix()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a new watcher for the test prefix.
	watcher, err := s.B.NewWatcher(ctx, backend.Watch{Prefixes: [][]byte{prefix("")}})
	c.Assert(err, check.IsNil)
	defer watcher.Close()

	// Make sure INIT event is emitted.
	select {
	case e := <-watcher.Events():
		c.Assert(e.Type, check.Equals, types.OpInit)
	case <-watcher.Done():
		c.Fatalf("Watcher has unexpectedly closed.")
	case <-time.After(2 * time.Second):
		c.Fatalf("Timeout waiting for event.")
	}

	// Add item to backend.
	item := &backend.Item{Key: prefix("b"), Value: []byte("val")}
	_, err = s.B.Put(ctx, *item)
	c.Assert(err, check.IsNil)

	// Make sure item was added into backend.
	item, err = s.B.Get(ctx, item.Key)
	c.Assert(err, check.IsNil)

	// Make sure a PUT event is emitted.
	select {
	case e := <-watcher.Events():
		c.Assert(e.Type, check.Equals, types.OpPut)
		c.Assert(string(e.Item.Key), check.Equals, string(item.Key))
		c.Assert(string(e.Item.Value), check.Equals, string(item.Value))
	case <-watcher.Done():
		c.Fatalf("Watcher has unexpectedly closed.")
	case <-time.After(2 * time.Second):
		c.Fatalf("Timeout waiting for event.")
	}

	// Delete item from backend.
	err = s.B.Delete(ctx, item.Key)
	c.Assert(err, check.IsNil)

	// Make sure item is no longer in backend.
	_, err = s.B.Get(ctx, item.Key)
	c.Assert(err, check.NotNil)

	// Make sure a DELETE event is emitted.
	select {
	case e := <-watcher.Events():
		c.Assert(e.Type, check.Equals, types.OpDelete)
		c.Assert(string(e.Item.Key), check.Equals, string(item.Key))
	case <-watcher.Done():
		c.Fatalf("Watcher has unexpectedly closed.")
	case <-time.After(2 * time.Second):
		c.Fatalf("Timeout waiting for event.")
	}

	// Add item to backend with a 1 second TTL.
	item = &backend.Item{
		Key:     prefix("c"),
		Value:   []byte("val"),
		Expires: s.Clock.Now().Add(1 * time.Second),
	}
	_, err = s.B.Put(ctx, *item)
	c.Assert(err, check.IsNil)

	// Make sure item was added into backend.
	item, err = s.B.Get(ctx, item.Key)
	c.Assert(err, check.IsNil)

	// Make sure a PUT event is emitted.
	select {
	case e := <-watcher.Events():
		c.Assert(e.Type, check.Equals, types.OpPut)
		c.Assert(string(e.Item.Key), check.Equals, string(item.Key))
		c.Assert(string(e.Item.Value), check.Equals, string(item.Value))
	case <-watcher.Done():
		c.Fatalf("Watcher has unexpectedly closed.")
	case <-time.After(2 * time.Second):
		c.Fatalf("Timeout waiting for event.")
	}

	// Wait a few seconds for the item to expire.
	s.Clock.Advance(3 * time.Second)

	// Make sure item has been removed.
	_, err = s.B.Get(ctx, item.Key)
	c.Assert(err, check.NotNil)

	// Make sure a DELETE event is emitted.
	select {
	case e := <-watcher.Events():
		c.Assert(e.Type, check.Equals, types.OpDelete)
		c.Assert(string(e.Item.Key), check.Equals, string(item.Key))
	case <-watcher.Done():
		c.Fatalf("Watcher has unexpectedly closed.")
	case <-time.After(2 * time.Second):
		c.Fatalf("Timeout waiting for event.")
	}
}

// WatchersClose tests scenarios with watches close
func (s *BackendSuite) WatchersClose(c *check.C) {
	prefix := MakePrefix()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b, err := s.NewBackend()
	c.Assert(err, check.IsNil)

	watcher, err := b.NewWatcher(ctx, backend.Watch{Prefixes: [][]byte{prefix("")}})
	c.Assert(err, check.IsNil)

	// cancel context -> get watcher to close
	cancel()

	select {
	case <-watcher.Done():
	case <-time.After(time.Second):
		c.Fatalf("Timeout waiting for watcher to close")
	}

	// closing backend should close associated watcher too
	watcher, err = b.NewWatcher(context.Background(), backend.Watch{Prefixes: [][]byte{prefix("")}})
	c.Assert(err, check.IsNil)

	b.Close()

	select {
	case <-watcher.Done():
	case <-time.After(time.Second):
		c.Fatalf("Timeout waiting for watcher to close")
	}
}

// Locking tests locking logic
func (s *BackendSuite) Locking(c *check.C, bk backend.Backend) {
	tok1 := "token1"
	tok2 := "token2"
	ttl := 5 * time.Second

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
				s.Clock.Advance(1 * time.Second)
			}
		}
	}()

	lock, err := backend.AcquireLock(ctx, bk, tok1, ttl)
	c.Assert(err, check.IsNil)
	x := int32(7)

	go func() {
		atomic.StoreInt32(&x, 9)
		c.Assert(lock.Release(ctx, bk), check.IsNil)
	}()
	lock, err = backend.AcquireLock(ctx, bk, tok1, ttl)
	c.Assert(err, check.IsNil)
	atomic.AddInt32(&x, 9)

	c.Assert(atomic.LoadInt32(&x), check.Equals, int32(18))
	c.Assert(lock.Release(ctx, bk), check.IsNil)

	lock, err = backend.AcquireLock(ctx, bk, tok1, ttl)
	c.Assert(err, check.IsNil)
	atomic.StoreInt32(&x, 7)
	go func() {
		atomic.StoreInt32(&x, 9)
		c.Assert(lock.Release(ctx, bk), check.IsNil)
	}()
	lock, err = backend.AcquireLock(ctx, bk, tok1, ttl)
	c.Assert(err, check.IsNil)
	atomic.AddInt32(&x, 9)
	c.Assert(atomic.LoadInt32(&x), check.Equals, int32(18))
	c.Assert(lock.Release(ctx, bk), check.IsNil)

	y := int32(0)
	lock1, err := backend.AcquireLock(ctx, bk, tok1, ttl)
	c.Assert(err, check.IsNil)
	lock2, err := backend.AcquireLock(ctx, bk, tok2, ttl)
	c.Assert(err, check.IsNil)
	go func() {
		atomic.StoreInt32(&y, 15)
		c.Assert(lock1.Release(ctx, bk), check.IsNil)
		c.Assert(lock2.Release(ctx, bk), check.IsNil)
	}()

	lock, err = backend.AcquireLock(ctx, bk, tok1, ttl)
	c.Assert(err, check.IsNil)
	c.Assert(atomic.LoadInt32(&y), check.Equals, int32(15))

	c.Assert(lock.Release(ctx, bk), check.IsNil)
}

// ConcurrentOperations tests concurrent operations on the same
// shared backend
func (s *BackendSuite) ConcurrentOperations(c *check.C) {
	l := s.B
	c.Assert(l, check.NotNil)
	l2 := s.B2
	c.Assert(l2, check.NotNil)

	prefix := MakePrefix()
	ctx := context.TODO()
	value1 := "this first value should not be corrupted by concurrent ops"
	value2 := "this second value should not be corrupted too"
	const attempts = 50
	resultsC := make(chan struct{}, attempts*4)
	for i := 0; i < attempts; i++ {
		go func(cnt int) {
			_, err := l.Put(ctx, backend.Item{Key: prefix("key"), Value: []byte(value1)})
			resultsC <- struct{}{}
			c.Assert(err, check.IsNil)
		}(i)

		go func(cnt int) {
			_, err := l2.CompareAndSwap(ctx,
				backend.Item{Key: prefix("key"), Value: []byte(value2)},
				backend.Item{Key: prefix("key"), Value: []byte(value1)})
			resultsC <- struct{}{}
			if err != nil && !trace.IsCompareFailed(err) {
				c.Assert(err, check.IsNil)
			}
		}(i)

		go func(cnt int) {
			_, err := l2.Create(ctx, backend.Item{Key: prefix("key"), Value: []byte(value2)})
			resultsC <- struct{}{}
			if err != nil && !trace.IsAlreadyExists(err) {
				c.Assert(err, check.IsNil)
			}
		}(i)

		go func(cnt int) {
			item, err := l.Get(ctx, prefix("key"))
			resultsC <- struct{}{}
			if err != nil && !trace.IsNotFound(err) {
				c.Assert(err, check.IsNil)
			}
			// make sure data is not corrupted along the way
			if err == nil {
				val := string(item.Value)
				if val != value1 && val != value2 {
					c.Fatalf("expected one of %q or %q and got %q", value1, value2, val)
				}
			}
		}(i)

		go func(cnt int) {
			err := l2.Delete(ctx, prefix("key"))
			if err != nil && !trace.IsNotFound(err) {
				c.Assert(err, check.IsNil)
			}
			resultsC <- struct{}{}
		}(i)
	}
	timeoutC := time.After(3 * time.Second)
	for i := 0; i < attempts*5; i++ {
		select {
		case <-resultsC:
		case <-timeoutC:
			c.Fatalf("timeout waiting for goroutines to finish")
		}
	}
}

// Mirror tests mirror mode for backends (used in caches). Only some backends
// support mirror mode (like memory).
func (s *BackendSuite) Mirror(c *check.C, b backend.Backend) {
	prefix := MakePrefix()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a new watcher for the test prefix.
	watcher, err := b.NewWatcher(ctx, backend.Watch{Prefixes: [][]byte{prefix("")}})
	c.Assert(err, check.IsNil)
	defer watcher.Close()

	// Make sure INIT event is emitted.
	select {
	case e := <-watcher.Events():
		c.Assert(e.Type, check.Equals, types.OpInit)
	case <-watcher.Done():
		c.Fatalf("Watcher has unexpectedly closed.")
	case <-time.After(2 * time.Second):
		c.Fatalf("Timeout waiting for event.")
	}

	// Add item to backend with a 1 second TTL.
	item := &backend.Item{
		Key:     prefix("a"),
		Value:   []byte("val"),
		Expires: time.Now().Add(1 * time.Second),
	}
	_, err = b.Put(ctx, *item)
	c.Assert(err, check.IsNil)

	// Make sure item was added into backend.
	item, err = b.Get(ctx, item.Key)
	c.Assert(err, check.IsNil)

	// Save the original ID, later in this test after an update, the ID should
	// not have changed in mirror mode.
	originalID := item.ID

	// Make sure a PUT event is emitted.
	select {
	case e := <-watcher.Events():
		c.Assert(e.Type, check.Equals, types.OpPut)
		c.Assert(string(e.Item.Key), check.Equals, string(item.Key))
		c.Assert(string(e.Item.Value), check.Equals, string(item.Value))
	case <-watcher.Done():
		c.Fatalf("Watcher has unexpectedly closed.")
	case <-time.After(2 * time.Second):
		c.Fatalf("Timeout waiting for event.")
	}

	// Wait 1 second for the item to expire.
	time.Sleep(time.Second)

	// Make sure item has not been removed.
	nitem, err := b.Get(ctx, item.Key)
	c.Assert(err, check.IsNil)
	c.Assert(string(item.Key), check.Equals, string(nitem.Key))
	c.Assert(string(item.Value), check.Equals, string(nitem.Value))

	// Make sure a DELETE event was not emitted.
	select {
	case e := <-watcher.Events():
		c.Fatalf("Received event: %v.", e)
	case <-watcher.Done():
		c.Fatalf("Watcher has unexpectedly closed.")
	case <-time.After(2 * time.Second):
	}

	// Update the existing item.
	_, err = b.Put(ctx, backend.Item{
		Key:   prefix("a"),
		Value: []byte("val2"),
	})
	c.Assert(err, check.IsNil)

	// Get update item and make sure that the ID has not changed.
	item, err = b.Get(ctx, prefix("a"))
	c.Assert(err, check.IsNil)
	c.Assert(item.ID, check.Equals, originalID)

	// Add item to backend that is already expired.
	item2 := &backend.Item{
		Key:     prefix("b"),
		Value:   []byte("val"),
		Expires: time.Now().Add(-1 * time.Second),
	}
	_, err = b.Put(ctx, *item2)
	c.Assert(err, check.IsNil)

	// Make sure item was added into backend despite being expired.
	_, err = b.Get(ctx, item2.Key)
	c.Assert(err, check.IsNil)
}

func (s *BackendSuite) addItem(ctx context.Context, c *check.C, key []byte, value string, expires time.Time) (backend.Item, backend.Lease) {
	item := backend.Item{
		Key:     key,
		Value:   []byte(value),
		Expires: expires,
	}
	lease, err := s.B.Put(ctx, item)
	c.Assert(err, check.IsNil)
	return item, *lease
}

// MakePrefix returns function that appends unique prefix
// to any key, used to make test suite concurrent-run proof
func MakePrefix() func(k string) []byte {
	id := "/" + uuid.New()
	return func(k string) []byte {
		return []byte(id + k)
	}
}

// ExpectItems tests that items equal to expected list
func ExpectItems(c *check.C, items, expected []backend.Item) {
	if len(items) != len(expected) {
		c.Fatalf("Expected %v items, got %v.", len(expected), len(items))
	}
	for i := range items {
		c.Assert(string(items[i].Key), check.Equals, string(expected[i].Key))
		c.Assert(string(items[i].Value), check.Equals, string(expected[i].Value))
	}
}

func verifyEvents(c *check.C, obtained, expected []backend.Event) {
	verifyIncreasingIDs(c, obtained)
	verifyNoDuplicateIDs(c, obtained)
	verifyExpireTimestampsIncreasing(c, obtained, expected)
}

func verifyIncreasingIDs(c *check.C, obtained []backend.Event) {
	lastID := int64(-1)
	for _, item := range obtained {
		c.Assert(item.Item.ID > lastID, check.Equals, true, check.Commentf("must be increasing"))
		lastID = item.Item.ID
	}
}

func verifyNoDuplicateIDs(c *check.C, obtained []backend.Event) {
	dedup := make(map[int64]struct{})
	for _, event := range obtained {
		if _, ok := dedup[event.Item.ID]; ok {
			c.Fatalf("Duplicate ID for %v.", event.Item.ID)
		}
		dedup[event.Item.ID] = struct{}{}
	}
}

func verifyExpireTimestampsIncreasing(c *check.C, obtained, expected []backend.Event) {
	c.Assert(obtained, check.HasLen, len(expected))
	for i := range expected {
		if obtained[i].Item.Expires.After(expected[i].Item.Expires) {
			c.Errorf("Expected %v >= %v",
				expected[i].Item.Expires,
				obtained[i].Item.Expires,
			)
		}
	}
}
