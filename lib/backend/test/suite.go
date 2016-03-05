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

// Package test contains a backend acceptance test suite that is backend implementation independant
// each backend will use the suite to test itself
package test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"

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
			c.Fatalf("Timeout occured waiting for events")
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
	keys, err := s.B.GetKeys([]string{"keys"})
	c.Assert(err, IsNil)
	c.Assert(keys, IsNil)

	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "bkey", []byte("val1"), 0), IsNil)
	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "akey", []byte("val2"), 0), IsNil)

	_, err = s.B.GetVal([]string{"a"}, "b")
	c.Assert(err, FitsTypeOf, &teleport.NotFoundError{})
	_, _, err = s.B.GetValAndTTL([]string{"a"}, "b")
	c.Assert(err, FitsTypeOf, &teleport.NotFoundError{})
	_, err = s.B.GetVal([]string{"a", "b"}, "x")
	c.Assert(err, FitsTypeOf, &teleport.NotFoundError{})
	_, _, err = s.B.GetValAndTTL([]string{"a", "b"}, "x")
	c.Assert(err, FitsTypeOf, &teleport.NotFoundError{})

	keys, _ = s.B.GetKeys([]string{"a", "b", "bkey"})
	c.Assert(len(keys), Equals, 0)

	keys, err = s.B.GetKeys([]string{"a", "b"})
	c.Assert(err, IsNil)
	c.Assert(keys, DeepEquals, []string{"akey", "bkey"})

	out, err := s.B.GetVal([]string{"a", "b"}, "bkey")
	c.Assert(err, IsNil)
	c.Assert(string(out), Equals, "val1")
	out, ttl, err := s.B.GetValAndTTL([]string{"a", "b"}, "bkey")
	c.Assert(err, IsNil)
	c.Assert(string(out), Equals, "val1")
	c.Assert(ttl, Equals, time.Duration(0))

	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "bkey", []byte("val-updated"), 0), IsNil)
	out, err = s.B.GetVal([]string{"a", "b"}, "bkey")
	c.Assert(err, IsNil)
	c.Assert(string(out), Equals, "val-updated")

	c.Assert(s.B.DeleteKey([]string{"a", "b"}, "bkey"), IsNil)
	c.Assert(s.B.DeleteKey([]string{"a", "b"}, "bkey"), FitsTypeOf, &teleport.NotFoundError{})
	_, err = s.B.GetVal([]string{"a", "b"}, "bkey")
	c.Assert(err, FitsTypeOf, &teleport.NotFoundError{})

	c.Assert(s.B.UpsertVal([]string{"a", "c"}, "xkey", []byte("val3"), 0), IsNil)
	c.Assert(s.B.UpsertVal([]string{"a", "c"}, "ykey", []byte("val4"), 0), IsNil)
	c.Assert(s.B.DeleteBucket([]string{"a"}, "c"), IsNil)
	_, err = s.B.GetVal([]string{"a", "c"}, "xkey")
	c.Assert(err, FitsTypeOf, &teleport.NotFoundError{})
	_, err = s.B.GetVal([]string{"a", "c"}, "ykey")
	c.Assert(err, FitsTypeOf, &teleport.NotFoundError{})

}

func (s *BackendSuite) CompareAndSwap(c *C) {
	prev, err := s.B.CompareAndSwap([]string{"a", "b"}, "bkey", []byte("val10"), 0, []byte("1231"))
	c.Assert(err, FitsTypeOf, &teleport.CompareFailedError{})
	c.Assert(len(prev), Equals, 0)

	prev, err = s.B.CompareAndSwap([]string{"a", "b"}, "bkey", []byte("val1"), 0, []byte{})
	c.Assert(err, IsNil)
	c.Assert(len(prev), Equals, 0)

	prev, err = s.B.CompareAndSwap([]string{"a", "b"}, "bkey", []byte("val2"), 0, []byte{})
	c.Assert(err, FitsTypeOf, &teleport.CompareFailedError{})
	c.Assert(string(prev), DeepEquals, "val1")

	prev, err = s.B.CompareAndSwap([]string{"a", "b"}, "bkey", []byte("val2"), 0, []byte("abcd"))
	c.Assert(err, FitsTypeOf, &teleport.CompareFailedError{})
	c.Assert(string(prev), DeepEquals, "val1")

	out, err := s.B.GetVal([]string{"a", "b"}, "bkey")
	c.Assert(err, IsNil)
	c.Assert(string(out), Equals, "val1")

	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "anotherkey", []byte("val3"), 0), IsNil)

	prev, err = s.B.CompareAndSwap([]string{"a", "b"}, "bkey", []byte("val4"), 0, []byte("val1"))
	c.Assert(err, IsNil)
	c.Assert(string(prev), DeepEquals, "val1")

	out, err = s.B.GetVal([]string{"a", "b"}, "bkey")
	c.Assert(err, IsNil)
	c.Assert(string(out), Equals, "val4")
}

func (s *BackendSuite) Expiration(c *C) {
	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "bkey", []byte("val1"), time.Second), IsNil)
	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "akey", []byte("val2"), 0), IsNil)

	time.Sleep(2 * time.Second)

	keys, err := s.B.GetKeys([]string{"a", "b"})
	c.Assert(err, IsNil)
	c.Assert(keys, DeepEquals, []string{"akey"})
}

func (s *BackendSuite) Renewal(c *C) {
	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "bkey", []byte("val1"), time.Second), IsNil)

	time.Sleep(time.Second)

	c.Assert(s.B.TouchVal([]string{"a", "b"}, "bkey", 100*time.Second), IsNil)

	val, ttl, err := s.B.GetValAndTTL([]string{"a", "b"}, "bkey")
	c.Assert(err, IsNil)
	c.Assert(string(val), Equals, "val1")
	c.Assert(ttl > time.Second, Equals, true)

	err = s.B.TouchVal([]string{"a", "b"}, "non-key", 100*time.Second)
	c.Assert(teleport.IsNotFound(err), Equals, true)
}

func (s *BackendSuite) Create(c *C) {
	c.Assert(s.B.CreateVal([]string{"a", "b"}, "bkey", []byte("val1"), time.Second), IsNil)
	err := s.B.CreateVal([]string{"a", "b"}, "bkey", []byte("val2"), 0)
	c.Assert(teleport.IsAlreadyExists(err), Equals, true)

	val, err := s.B.GetVal([]string{"a", "b"}, "bkey")
	c.Assert(err, IsNil)
	c.Assert(string(val), Equals, "val1")
}

func (s *BackendSuite) ValueAndTTl(c *C) {
	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "bkey",
		[]byte("val1"), 2000*time.Millisecond), IsNil)

	time.Sleep(1000 * time.Millisecond)

	value, ttl, err := s.B.GetValAndTTL([]string{"a", "b"}, "bkey")
	c.Assert(err, IsNil)
	c.Assert(string(value), DeepEquals, "val1")
	ttlIsRight := (ttl < 1400*time.Millisecond) && (ttl > 600*time.Millisecond)
	c.Assert(ttlIsRight, Equals, true)
}

func (s *BackendSuite) Locking(c *C) {
	tok1 := "token1"
	tok2 := "token2"

	c.Assert(s.B.ReleaseLock(tok1), FitsTypeOf, &teleport.NotFoundError{})

	c.Assert(s.B.AcquireLock(tok1, time.Second), IsNil)
	x := int32(7)

	go func() {
		atomic.StoreInt32(&x, 9)
		c.Assert(s.B.ReleaseLock(tok1), IsNil)
	}()
	c.Assert(s.B.AcquireLock(tok1, 0), IsNil)
	atomic.AddInt32(&x, 9)

	c.Assert(atomic.LoadInt32(&x), Equals, int32(18))
	c.Assert(s.B.ReleaseLock(tok1), IsNil)

	c.Assert(s.B.AcquireLock(tok1, 0), IsNil)
	atomic.StoreInt32(&x, 7)
	go func() {
		atomic.StoreInt32(&x, 9)
		c.Assert(s.B.ReleaseLock(tok1), IsNil)
	}()
	c.Assert(s.B.AcquireLock(tok1, 0), IsNil)
	atomic.AddInt32(&x, 9)
	c.Assert(atomic.LoadInt32(&x), Equals, int32(18))
	c.Assert(s.B.ReleaseLock(tok1), IsNil)

	y := int32(0)
	c.Assert(s.B.AcquireLock(tok1, 0), IsNil)
	c.Assert(s.B.AcquireLock(tok2, 0), IsNil)
	go func() {
		atomic.StoreInt32(&y, 15)
		c.Assert(s.B.ReleaseLock(tok1), IsNil)
		c.Assert(s.B.ReleaseLock(tok2), IsNil)
	}()

	c.Assert(s.B.AcquireLock(tok1, 0), IsNil)
	c.Assert(atomic.LoadInt32(&y), Equals, int32(15))

	c.Assert(s.B.ReleaseLock(tok1), IsNil)
	c.Assert(s.B.ReleaseLock(tok1), FitsTypeOf, &teleport.NotFoundError{})
}

func toSet(vals []string) map[string]struct{} {
	out := make(map[string]struct{}, len(vals))
	for _, v := range vals {
		out[v] = struct{}{}
	}
	return out
}
