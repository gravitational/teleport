/*
Copyright 2017 Gravitational, Inc.

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

package state

import (
	"context"
	"time"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"gopkg.in/check.v1"
)

type CacheLogSuite struct {
	clock clockwork.Clock
}

var _ = check.Suite(&CacheLogSuite{})

func (s *CacheLogSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
	s.clock = clockwork.NewFakeClock()
}

func newLog(cfg CachingAuditLogConfig) *CachingAuditLog {
	cfg.Namespace = "ns"
	cfg.SessionID = "s1"
	auditLog, err := NewCachingAuditLog(cfg)
	if err != nil {
		panic(err)
	}
	return auditLog
}

func (s *CacheLogSuite) newSlice(data string) *events.SessionSlice {
	chunk := &events.SessionChunk{
		Time: s.clock.Now().UnixNano(),
		Data: []byte(data),
	}
	return &events.SessionSlice{
		Namespace: "ns",
		SessionID: "s1",
		Chunks:    []*events.SessionChunk{chunk},
		Version:   events.V2,
	}
}

// TestFlushTail tests scenario when the buffer is not filled up
// and the forwarder flushes it
func (s *CacheLogSuite) TestFlushTail(c *check.C) {
	mock := events.NewMockAuditLog(0)
	log := newLog(CachingAuditLogConfig{
		FlushTimeout: time.Millisecond,
		FlushChunks:  100,
		QueueLen:     200,
		Server:       mock,
	})
	slice := s.newSlice("hello")
	err := log.PostSessionSlice(*slice)
	c.Assert(err, check.IsNil)
	var out *events.SessionSlice
	select {
	case out = <-mock.SlicesC:
	case <-time.After(time.Second):
		c.Fatalf("timeout")
	}
	fixtures.DeepCompare(c, out, slice)
}

// TestFlushOnClose tests that log forwarder flushes
// on log close and we don't loose any events
func (s *CacheLogSuite) TestFlushOnClose(c *check.C) {
	mock := events.NewMockAuditLog(0)
	log := newLog(CachingAuditLogConfig{
		FlushTimeout: 10 * time.Second,
		FlushChunks:  100,
		QueueLen:     200,
		Server:       mock,
	})
	slice := s.newSlice("hello")
	err := log.PostSessionSlice(*slice)
	c.Assert(err, check.IsNil)
	err = log.Close()
	c.Assert(err, check.IsNil)
	var out *events.SessionSlice
	select {
	case out = <-mock.SlicesC:
	case <-time.After(time.Second):
		c.Fatalf("timeout")
	}
	fixtures.DeepCompare(c, out, slice)
}

// TestFlushWait makes sure wait returns correctly
// after audit log closes and the event is received
func (s *CacheLogSuite) TestFlushWait(c *check.C) {
	mock := events.NewMockAuditLog(1)
	log := newLog(CachingAuditLogConfig{
		FlushTimeout: 10 * time.Second,
		FlushChunks:  100,
		QueueLen:     200,
		Server:       mock,
	})
	slice := s.newSlice("hello")
	err := log.PostSessionSlice(*slice)
	c.Assert(err, check.IsNil)
	err = log.Close()
	c.Assert(err, check.IsNil)
	wait, cancel := context.WithTimeout(context.TODO(), time.Second)
	defer cancel()
	err = log.WaitForDelivery(wait)
	c.Assert(err, check.IsNil)

	var out *events.SessionSlice
	select {
	case out = <-mock.SlicesC:
	case <-time.After(time.Second):
		c.Fatalf("timeout")
	}
	fixtures.DeepCompare(c, out, slice)
}

// TestThrottleTimeout tests that we throttle and timeout
func (s *CacheLogSuite) TestThrottleTimeout(c *check.C) {
	mock := events.NewMockAuditLog(0)
	log := newLog(CachingAuditLogConfig{
		ThrottleTimeout:  time.Millisecond,
		ThrottleDuration: 10 * time.Second,
		Server:           mock,
	})
	slice := s.newSlice("hello")
	doneC := make(chan struct{})
	go func() {
		err := log.PostSessionSlice(*slice)
		c.Assert(err, check.IsNil)
		doneC <- struct{}{}
	}()
	select {
	case <-time.After(time.Second):
		c.Fatalf("timeout")
	case <-doneC:
	}
}

func waitSlice(c *check.C, in chan *events.SessionSlice, timeout time.Duration) *events.SessionSlice {
	select {
	case <-time.After(timeout):
		c.Fatalf("timeout")
		return nil
	case out := <-in:
		return out
	}
}

// TestFlushChunks tests that we flush on chunks fill up
func (s *CacheLogSuite) TestFlushChunks(c *check.C) {
	mock := events.NewMockAuditLog(2)
	log := newLog(CachingAuditLogConfig{
		FlushTimeout: 10 * time.Second,
		FlushChunks:  2,
		Server:       mock,
	})
	a, b := s.newSlice("1"), s.newSlice("2")
	err := log.PostSessionSlice(*a)
	c.Assert(err, check.IsNil)
	err = log.PostSessionSlice(*b)
	c.Assert(err, check.IsNil)

	out := waitSlice(c, mock.SlicesC, time.Second)
	fixtures.DeepCompare(c, out.Chunks[0], a.Chunks[0])
	fixtures.DeepCompare(c, out.Chunks[1], b.Chunks[0])
}

// TestFlushBytes tests that we flush on bytes count, rather than
// chunks count
func (s *CacheLogSuite) TestFlushBytes(c *check.C) {
	mock := events.NewMockAuditLog(2)
	log := newLog(CachingAuditLogConfig{
		FlushTimeout: 10 * time.Second,
		FlushChunks:  100,
		FlushBytes:   1,
		Server:       mock,
	})
	slice := s.newSlice("howdy")
	err := log.PostSessionSlice(*slice)
	c.Assert(err, check.IsNil)

	out := waitSlice(c, mock.SlicesC, time.Second)
	fixtures.DeepCompare(c, out, slice)
}

// TestRetryOnError tests that forwarder goroutine
// retries on errors and eventually delivers the message
func (s *CacheLogSuite) TestRetryOnError(c *check.C) {
	mock := events.NewMockAuditLog(2)
	mock.SetError(trace.ConnectionProblem(nil, "oops"))
	log := newLog(CachingAuditLogConfig{
		FlushTimeout:           10 * time.Second,
		FlushChunks:            1,
		Server:                 mock,
		BackoffInitialInterval: time.Millisecond,
	})
	slice := s.newSlice("howdy")
	err := log.PostSessionSlice(*slice)
	c.Assert(err, check.IsNil)
	// wait for a couple of failed attempt to deliver
	for i := 0; i < 3; i++ {
		waitSlice(c, mock.FailedAttemptsC, time.Second)
	}
	// wait until successful attempt
	mock.SetError(nil)
	out := waitSlice(c, mock.SlicesC, time.Second)
	fixtures.DeepCompare(c, out, slice)
}
