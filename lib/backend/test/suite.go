/*
Copyright 2015 Gravitational, Inc.

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
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/fixtures"

	"github.com/gravitational/trace"
	. "gopkg.in/check.v1"
)

func TestBackend(t *testing.T) { TestingT(t) }

type BackendSuite struct {
	B        backend.Backend
	ChangesC chan interface{}
}

func (s *BackendSuite) collectChanges(c *C, expected int) []interface{} {
	changes := make([]interface{}, expected)
	for i, _ := range changes {
		select {
		case changes[i] = <-s.ChangesC:
			// successfully collected changes
		case <-time.After(2 * time.Second):
			c.Fatalf("Timeout occurred waiting for events")
		}
	}
	return changes
}

func (s *BackendSuite) expectChanges(c *C, expected ...interface{}) {
	changes := s.collectChanges(c, len(expected))
	for i, ch := range changes {
		c.Assert(ch, DeepEquals, expected[i])
	}
}

func (s *BackendSuite) BasicCRUD(c *C) {
	bucket := []string{"test", "create"}
	c.Assert(s.B.CreateVal(bucket, "one", []byte("1"), backend.Forever), IsNil)
	err := s.B.CreateVal(bucket, "one", []byte("2"), backend.Forever)
	c.Assert(trace.IsAlreadyExists(err), Equals, true)

	val, err := s.B.GetVal(bucket, "one")
	c.Assert(err, IsNil)
	c.Assert(string(val), Equals, "1")

	keys, err := s.B.GetKeys([]string{"keys"})
	c.Assert(err, IsNil)
	c.Assert(keys, HasLen, 0)

	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "bkey", []byte("val1"), 0), IsNil)
	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "akey", []byte("val2"), 0), IsNil)

	_, err = s.B.GetVal([]string{"a"}, "b")
	c.Assert(trace.IsBadParameter(err), Equals, true, Commentf("%#v", err))
	_, err = s.B.GetVal([]string{"a"}, "b")
	c.Assert(trace.IsBadParameter(err), Equals, true, Commentf("%#v", err))
	_, err = s.B.GetVal([]string{"a", "b"}, "x")
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))
	_, err = s.B.GetVal([]string{"a", "b"}, "x")
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))

	keys, _ = s.B.GetKeys([]string{"a", "b", "bkey"})
	c.Assert(len(keys), Equals, 0)

	keys, err = s.B.GetKeys([]string{"a", "b"})
	c.Assert(err, IsNil)
	c.Assert(keys, DeepEquals, []string{"akey", "bkey"})

	out, err := s.B.GetVal([]string{"a", "b"}, "bkey")
	c.Assert(err, IsNil)
	c.Assert(string(out), Equals, "val1")
	out, err = s.B.GetVal([]string{"a", "b"}, "bkey")
	c.Assert(err, IsNil)
	c.Assert(string(out), Equals, "val1")

	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "bkey", []byte("val-updated"), 0), IsNil)
	out, err = s.B.GetVal([]string{"a", "b"}, "bkey")
	c.Assert(err, IsNil)
	c.Assert(string(out), Equals, "val-updated")

	c.Assert(s.B.DeleteKey([]string{"a", "b"}, "bkey"), IsNil)
	c.Assert(trace.IsNotFound(s.B.DeleteKey([]string{"a", "b"}, "bkey")), Equals, true, Commentf("%#v", err))
	_, err = s.B.GetVal([]string{"a", "b"}, "bkey")
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))

	c.Assert(s.B.UpsertVal([]string{"a", "c"}, "xkey", []byte("val3"), 0), IsNil)
	c.Assert(s.B.UpsertVal([]string{"a", "c"}, "ykey", []byte("val4"), 0), IsNil)
	c.Assert(s.B.DeleteBucket([]string{"a"}, "c"), IsNil)
	_, err = s.B.GetVal([]string{"a", "c"}, "xkey")
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))
	_, err = s.B.GetVal([]string{"a", "c"}, "ykey")
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))
}

// CompareAndSwap tests compare and swap functionality
func (s *BackendSuite) CompareAndSwap(c *C) {
	bucket := []string{"test", "cas"}

	// compare and swap on non existing operation will fail
	err := s.B.CompareAndSwapVal(bucket, "one", []byte("1"), []byte("2"), backend.Forever)
	fixtures.ExpectCompareFailed(c, err)

	err = s.B.CreateVal(bucket, "one", []byte("1"), backend.Forever)
	c.Assert(err, IsNil)

	// success CAS!
	err = s.B.CompareAndSwapVal(bucket, "one", []byte("2"), []byte("1"), backend.Forever)
	c.Assert(err, IsNil)

	val, err := s.B.GetVal(bucket, "one")
	c.Assert(err, IsNil)
	c.Assert(string(val), Equals, "2")

	// value has been updated - not '1' any more
	err = s.B.CompareAndSwapVal(bucket, "one", []byte("3"), []byte("1"), backend.Forever)
	fixtures.ExpectCompareFailed(c, err)

	// existing value has not been changed by the failed CAS operation
	val, err = s.B.GetVal(bucket, "one")
	c.Assert(err, IsNil)
	c.Assert(string(val), Equals, "2")
}

// BatchCRUD tests batch CRUD operations if supported by the backend
func (s *BackendSuite) BatchCRUD(c *C) {
	getter, ok := s.B.(backend.ItemsGetter)
	if !ok {
		c.Skip("backend does not support batch get")
		return
	}
	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "bkey", []byte("val1"), 0), IsNil)
	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "akey", []byte("val2"), 0), IsNil)

	items, err := getter.GetItems([]string{"a", "b"})
	c.Assert(err, IsNil)
	c.Assert(len(items), Equals, 2)
	c.Assert(string(items[0].Value), Equals, "val2")
	c.Assert(items[0].Key, Equals, "akey")
	c.Assert(string(items[1].Value), Equals, "val1")
	c.Assert(items[1].Key, Equals, "bkey")
}

// Directories checks directories access
func (s *BackendSuite) Directories(c *C) {
	bucket := []string{"level1", "level2", "level3"}
	c.Assert(s.B.UpsertVal(bucket, "key", []byte("val"), 0), IsNil)

	keys, err := s.B.GetKeys(bucket[:2])
	c.Assert(err, IsNil)
	c.Assert(keys, DeepEquals, []string{"level3"})

	keys, err = s.B.GetKeys(bucket[:1])
	c.Assert(err, IsNil)
	c.Assert(keys, DeepEquals, []string{"level2"})
}

func (s *BackendSuite) Expiration(c *C) {
	bucket := []string{"one", "two"}
	c.Assert(s.B.UpsertVal(bucket, "bkey", []byte("val1"), time.Second), IsNil)
	c.Assert(s.B.UpsertVal(bucket, "akey", []byte("val2"), 0), IsNil)

	time.Sleep(2 * time.Second)

	keys, err := s.B.GetKeys(bucket)
	c.Assert(err, IsNil)
	c.Assert(keys, DeepEquals, []string{"akey"})
}

func (s *BackendSuite) ValueAndTTL(c *C) {
	bucket := []string{"test", "ttl"}
	c.Assert(s.B.UpsertVal(bucket, "bkey",
		[]byte("val1"), 2*time.Second), IsNil)

	time.Sleep(time.Second)

	value, err := s.B.GetVal(bucket, "bkey")
	c.Assert(err, IsNil)
	c.Assert(string(value), DeepEquals, "val1")
}

func (s *BackendSuite) Locking(c *C) {
	tok1 := "token1"
	tok2 := "token2"
	ttl := time.Second * 5

	err := s.B.ReleaseLock(tok1)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))

	c.Assert(s.B.AcquireLock(tok1, ttl), IsNil)
	x := int32(7)

	go func() {
		atomic.StoreInt32(&x, 9)
		c.Assert(s.B.ReleaseLock(tok1), IsNil)
	}()
	c.Assert(s.B.AcquireLock(tok1, ttl), IsNil)
	atomic.AddInt32(&x, 9)

	c.Assert(atomic.LoadInt32(&x), Equals, int32(18))
	c.Assert(s.B.ReleaseLock(tok1), IsNil)

	c.Assert(s.B.AcquireLock(tok1, ttl), IsNil)
	atomic.StoreInt32(&x, 7)
	go func() {
		atomic.StoreInt32(&x, 9)
		c.Assert(s.B.ReleaseLock(tok1), IsNil)
	}()
	c.Assert(s.B.AcquireLock(tok1, ttl), IsNil)
	atomic.AddInt32(&x, 9)
	c.Assert(atomic.LoadInt32(&x), Equals, int32(18))
	c.Assert(s.B.ReleaseLock(tok1), IsNil)

	y := int32(0)
	c.Assert(s.B.AcquireLock(tok1, ttl), IsNil)
	c.Assert(s.B.AcquireLock(tok2, ttl), IsNil)
	go func() {
		atomic.StoreInt32(&y, 15)
		c.Assert(s.B.ReleaseLock(tok1), IsNil)
		c.Assert(s.B.ReleaseLock(tok2), IsNil)
	}()

	c.Assert(s.B.AcquireLock(tok1, ttl), IsNil)
	c.Assert(atomic.LoadInt32(&y), Equals, int32(15))

	c.Assert(s.B.ReleaseLock(tok1), IsNil)
	err = s.B.ReleaseLock(tok1)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%T", err))
}

func toSet(vals []string) map[string]struct{} {
	out := make(map[string]struct{}, len(vals))
	for _, v := range vals {
		out[v] = struct{}{}
	}
	return out
}
