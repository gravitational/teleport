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
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/fixtures"

	//	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"gopkg.in/check.v1"
)

var _ = fmt.Printf

func TestBackend(t *testing.T) { check.TestingT(t) }

type BackendSuite struct {
	B          backend.Backend
	NewBackend func() (backend.Backend, error)
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

	// put new item suceeds
	item = backend.Item{Key: prefix("/put"), Value: []byte("world")}
	_, err = s.B.Put(ctx, item)
	c.Assert(err, check.IsNil)

	out, err = s.B.Get(ctx, item.Key)
	c.Assert(err, check.IsNil)
	c.Assert(string(out.Value), check.Equals, string(item.Value))
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

	_, err = s.B.Put(ctx, backend.Item{Key: prefix("b"), Value: []byte("val1"), Expires: time.Now().Add(time.Second)})
	c.Assert(err, check.IsNil)

	var items []backend.Item
	for i := 0; i < 4; i++ {
		time.Sleep(time.Second)
		res, err := s.B.GetRange(ctx, prefix(""), backend.RangeEnd(prefix("")), backend.NoLimit)
		c.Assert(err, check.IsNil)
		if len(res.Items) == 1 {
			items = res.Items
			break
		}
	}
	ExpectItems(c, items, []backend.Item{itemA})
}

// addSeconds adds seconds with a seconds precission
// always rounding up to the next second,
// because TTL engines are usually 1 second precision
func addSeconds(t time.Time, seconds int64) time.Time {
	return time.Unix(t.UTC().Unix()+seconds+1, 0)
}

// KeepAlive tests keep alive API
func (s *BackendSuite) KeepAlive(c *check.C) {
	prefix := MakePrefix()
	ctx := context.Background()

	item := backend.Item{Key: prefix("key"), Value: []byte("val1"), Expires: addSeconds(time.Now(), 2)}
	lease, err := s.B.Put(ctx, item)
	c.Assert(err, check.IsNil)

	time.Sleep(time.Second)

	// make sure that the value has not expired
	out, err := s.B.Get(ctx, item.Key)
	c.Assert(err, check.IsNil)
	c.Assert(string(out.Value), check.Equals, string(item.Value))
	c.Assert(string(out.Key), check.Equals, string(item.Key))

	err = s.B.KeepAlive(ctx, *lease, addSeconds(time.Now(), 2))
	c.Assert(err, check.IsNil)

	// should have expired if not keep alive
	diff := addSeconds(time.Now(), 1).Sub(time.Now())
	time.Sleep(diff + 100*time.Millisecond)

	out, err = s.B.Get(ctx, item.Key)
	c.Assert(err, check.IsNil)
	c.Assert(string(out.Value), check.Equals, string(item.Value))
	c.Assert(string(out.Key), check.Equals, string(item.Key))

	err = s.B.Delete(ctx, item.Key)
	c.Assert(err, check.IsNil)

	_, err = s.B.Get(ctx, item.Key)
	fixtures.ExpectNotFound(c, err)

	// keep alive on deleted or expired object should fail
	err = s.B.KeepAlive(ctx, *lease, addSeconds(time.Now(), 2))
	fixtures.ExpectNotFound(c, err)
}

// Events tests scenarios with event watches
func (s *BackendSuite) Events(c *check.C) {
	prefix := MakePrefix()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watcher, err := s.B.NewWatcher(ctx, backend.Watch{Prefix: prefix("")})
	c.Assert(err, check.IsNil)
	defer watcher.Close()

	item := &backend.Item{Key: prefix("b"), Value: []byte("val")}
	_, err = s.B.Put(ctx, *item)
	c.Assert(err, check.IsNil)

	item, err = s.B.Get(ctx, item.Key)
	c.Assert(err, check.IsNil)

	select {
	case e := <-watcher.Events():
		c.Assert(string(e.Item.Key), check.Equals, string(item.Key))
		c.Assert(string(e.Item.Value), check.Equals, string(item.Value))
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

	watcher, err := b.NewWatcher(ctx, backend.Watch{Prefix: prefix("")})
	c.Assert(err, check.IsNil)

	// cancel context -> get watcher to close
	cancel()

	select {
	case <-watcher.Done():
	case <-time.After(time.Second):
		c.Fatalf("Timeout waiting for watcher to close")
	}

	// closing backend should close associated watcher too
	watcher, err = b.NewWatcher(context.Background(), backend.Watch{Prefix: prefix("")})
	c.Assert(err, check.IsNil)

	b.Close()

	select {
	case <-watcher.Done():
	case <-time.After(time.Second):
		c.Fatalf("Timeout waiting for watcher to close")
	}
}

// Locking tests locking logic
func (s *BackendSuite) Locking(c *check.C) {
	tok1 := "token1"
	tok2 := "token2"
	ttl := time.Second * 5

	ctx := context.TODO()

	err := backend.ReleaseLock(ctx, s.B, tok1)
	fixtures.ExpectNotFound(c, err)

	c.Assert(backend.AcquireLock(ctx, s.B, tok1, ttl), check.IsNil)
	x := int32(7)

	go func() {
		atomic.StoreInt32(&x, 9)
		c.Assert(backend.ReleaseLock(ctx, s.B, tok1), check.IsNil)
	}()
	c.Assert(backend.AcquireLock(ctx, s.B, tok1, ttl), check.IsNil)
	atomic.AddInt32(&x, 9)

	c.Assert(atomic.LoadInt32(&x), check.Equals, int32(18))
	c.Assert(backend.ReleaseLock(ctx, s.B, tok1), check.IsNil)

	c.Assert(backend.AcquireLock(ctx, s.B, tok1, ttl), check.IsNil)
	atomic.StoreInt32(&x, 7)
	go func() {
		atomic.StoreInt32(&x, 9)
		c.Assert(backend.ReleaseLock(ctx, s.B, tok1), check.IsNil)
	}()
	c.Assert(backend.AcquireLock(ctx, s.B, tok1, ttl), check.IsNil)
	atomic.AddInt32(&x, 9)
	c.Assert(atomic.LoadInt32(&x), check.Equals, int32(18))
	c.Assert(backend.ReleaseLock(ctx, s.B, tok1), check.IsNil)

	y := int32(0)
	c.Assert(backend.AcquireLock(ctx, s.B, tok1, ttl), check.IsNil)
	c.Assert(backend.AcquireLock(ctx, s.B, tok2, ttl), check.IsNil)
	go func() {
		atomic.StoreInt32(&y, 15)
		c.Assert(backend.ReleaseLock(ctx, s.B, tok1), check.IsNil)
		c.Assert(backend.ReleaseLock(ctx, s.B, tok2), check.IsNil)
	}()

	c.Assert(backend.AcquireLock(ctx, s.B, tok1, ttl), check.IsNil)
	c.Assert(atomic.LoadInt32(&y), check.Equals, int32(15))

	c.Assert(backend.ReleaseLock(ctx, s.B, tok1), check.IsNil)
	err = backend.ReleaseLock(ctx, s.B, tok1)
	fixtures.ExpectNotFound(c, err)
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

func toSet(vals []string) map[string]struct{} {
	out := make(map[string]struct{}, len(vals))
	for _, v := range vals {
		out[v] = struct{}{}
	}
	return out
}
