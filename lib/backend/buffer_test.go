/*
Copyright 2018 Gravitational, Inc.

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

package backend

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	check "gopkg.in/check.v1"
)

func TestInit(t *testing.T) { check.TestingT(t) }

type BufferSuite struct{}

var _ = check.Suite(&BufferSuite{})

func (s *BufferSuite) SetUpSuite(c *check.C) {
	log.StandardLogger().Hooks = make(log.LevelHooks)
	formatter := &trace.TextFormatter{DisableTimestamp: false}
	log.SetFormatter(formatter)
	if testing.Verbose() {
		log.SetLevel(log.DebugLevel)
		log.SetOutput(os.Stdout)
	}
}

func (s *BufferSuite) list(c *check.C, bufferSize int, listSize int) {
	b, err := NewCircularBuffer(context.Background(), bufferSize)
	c.Assert(err, check.IsNil)
	defer b.Close()

	// empty by default
	expectEvents(c, b, nil)

	elements := makeIDs(listSize)

	// push through all elements of the list and make sure
	// the slice always matches
	for i := 0; i < len(elements); i++ {
		b.Push(Event{Item: Item{ID: elements[i]}})
		sliceEnd := i + 1 - bufferSize
		if sliceEnd < 0 {
			sliceEnd = 0
		}
		expectEvents(c, b, elements[sliceEnd:i+1])
	}

}

// TestBufferSizes tests various combinations of various
// buffer sizes and lists
func (s *BufferSuite) TestBufferSizes(c *check.C) {
	s.list(c, 1, 100)
	s.list(c, 2, 100)
	s.list(c, 3, 100)
	s.list(c, 4, 100)
}

// TestWatcher tests scenarios with watchers
func (s *BufferSuite) TestWatcherSimple(c *check.C) {
	ctx := context.TODO()
	b, err := NewCircularBuffer(ctx, 3)
	c.Assert(err, check.IsNil)
	defer b.Close()

	w, err := b.NewWatcher(ctx, Watch{})
	c.Assert(err, check.IsNil)
	defer w.Close()

	select {
	case e := <-w.Events():
		c.Assert(e.Type, check.Equals, OpInit)
	case <-time.After(100 * time.Millisecond):
		c.Fatalf("timeout waiting for event")
	}

	b.Push(Event{Item: Item{ID: 1}})

	select {
	case e := <-w.Events():
		c.Assert(e.Item.ID, check.Equals, int64(1))
	case <-time.After(100 * time.Millisecond):
		c.Fatalf("timeout waiting for event")
	}

	b.Close()
	b.Push(Event{Item: Item{ID: 2}})

	select {
	case <-w.Done():
		// expected
	case <-w.Events():
		c.Fatalf("unexpected event")
	case <-time.After(100 * time.Millisecond):
		c.Fatalf("timeout waiting for event")
	}
}

func makeIDs(size int) []int64 {
	out := make([]int64, size)
	for i := 0; i < size; i++ {
		out[i] = int64(i)
	}
	return out
}

func expectEvents(c *check.C, b *CircularBuffer, ids []int64) {
	events := b.Events()
	if len(ids) == 0 {
		c.Assert(len(events), check.Equals, 0)
		return
	}
	c.Assert(toIDs(events), check.DeepEquals, ids)
}

func toIDs(e []Event) []int64 {
	var out []int64
	for i := 0; i < len(e); i++ {
		out = append(out, e[i].Item.ID)
	}
	return out
}
