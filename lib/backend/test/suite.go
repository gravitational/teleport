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

	"github.com/gravitational/teleport/lib/backend"

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
	c.Assert(trace.IsNotFound(s.B.DeleteKey([]string{"a", "b"}, "bkey")), Equals, true)
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

func (s *BackendSuite) CompareAndSwap(c *C) {
	prev, err := s.B.CompareAndSwap([]string{"a", "b"}, "bkey", []byte("val10"), 0, []byte("1231"))
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))
	c.Assert(len(prev), Equals, 0)

	prev, err = s.B.CompareAndSwap([]string{"a", "b"}, "bkey", []byte("val1"), 0, []byte{})
	c.Assert(err, IsNil)
	c.Assert(len(prev), Equals, 0)

	_, err = s.B.CompareAndSwap([]string{"a", "b"}, "bkey", []byte("val2"), 0, []byte{})
	c.Assert(trace.IsAlreadyExists(err), Equals, true, Commentf("%#v", err))

	_, err = s.B.CompareAndSwap([]string{"a", "b"}, "bkey", []byte("val2"), 0, []byte("abcd"))
	c.Assert(trace.IsCompareFailed(err), Equals, true, Commentf("%#v", err))

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

func (s *BackendSuite) Create(c *C) {
	c.Assert(s.B.CreateVal([]string{"a", "b"}, "bkey", []byte("val1"), time.Second), IsNil)
	err := s.B.CreateVal([]string{"a", "b"}, "bkey", []byte("val2"), 0)
	c.Assert(trace.IsAlreadyExists(err), Equals, true)

	val, err := s.B.GetVal([]string{"a", "b"}, "bkey")
	c.Assert(err, IsNil)
	c.Assert(string(val), Equals, "val1")
}

func (s *BackendSuite) ValueAndTTl(c *C) {
	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "bkey",
		[]byte("val1"), 2000*time.Millisecond), IsNil)

	time.Sleep(1000 * time.Millisecond)

	value, err := s.B.GetVal([]string{"a", "b"}, "bkey")
	c.Assert(err, IsNil)
	c.Assert(string(value), DeepEquals, "val1")
}

func (s *BackendSuite) Locking(c *C) {
	tok1 := "token1"
	tok2 := "token2"

	err := s.B.ReleaseLock(tok1)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))

	c.Assert(s.B.AcquireLock(tok1, time.Second*100), IsNil)
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
