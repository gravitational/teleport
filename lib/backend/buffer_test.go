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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"gopkg.in/check.v1"
)

func TestInit(t *testing.T) { check.TestingT(t) }

type BufferSuite struct{}

var _ = check.Suite(&BufferSuite{})

func (s *BufferSuite) SetUpSuite(_ *check.C) {
	log.StandardLogger().Hooks = make(log.LevelHooks)
	log.SetFormatter(utils.NewDefaultTextFormatter(trace.IsTerminal(os.Stderr)))
	if testing.Verbose() {
		log.SetLevel(log.DebugLevel)
		log.SetOutput(os.Stdout)
	}
}

func (s *BufferSuite) list(c *check.C, bufferSize int, listSize int) {
	b := NewCircularBuffer(
		BufferCapacity(bufferSize),
	)
	defer b.Close()
	b.SetInit()
	s.listWithBuffer(c, b, bufferSize, listSize)
}

func (s *BufferSuite) listWithBuffer(c *check.C, b *CircularBuffer, bufferSize int, listSize int) {
	// empty by default
	expectEvents(c, b, nil)

	elements := makeIDs(listSize)

	// push through all elements of the list and make sure
	// the slice always matches
	for i := 0; i < len(elements); i++ {
		b.Emit(Event{Item: Item{ID: elements[i]}})
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

// TestBufferSizesReset tests various combinations of various
// buffer sizes and lists with clear.
func (s *BufferSuite) TestBufferSizesReset(c *check.C) {
	b := NewCircularBuffer(
		BufferCapacity(1),
	)
	defer b.Close()
	b.SetInit()

	s.listWithBuffer(c, b, 1, 100)
	b.Clear()
	s.listWithBuffer(c, b, 1, 100)
}

// TestWatcherSimple tests scenarios with watchers
func (s *BufferSuite) TestWatcherSimple(c *check.C) {
	ctx := context.Background()
	b := NewCircularBuffer(
		BufferCapacity(3),
	)
	defer b.Close()
	b.SetInit()

	w, err := b.NewWatcher(ctx, Watch{})
	c.Assert(err, check.IsNil)
	defer w.Close()

	select {
	case e := <-w.Events():
		c.Assert(e.Type, check.Equals, types.OpInit)
	case <-time.After(100 * time.Millisecond):
		c.Fatalf("Timeout waiting for event.")
	}

	b.Emit(Event{Item: Item{Key: []byte{Separator}, ID: 1}})

	select {
	case e := <-w.Events():
		c.Assert(e.Item.ID, check.Equals, int64(1))
	case <-time.After(100 * time.Millisecond):
		c.Fatalf("Timeout waiting for event.")
	}

	b.Close()
	b.Emit(Event{Item: Item{ID: 2}})

	select {
	case <-w.Done():
		// expected
	case <-w.Events():
		c.Fatalf("unexpected event")
	case <-time.After(100 * time.Millisecond):
		c.Fatalf("Timeout waiting for event.")
	}
}

// TestWatcherCapacity checks various watcher capacity scenarios
func (s *BufferSuite) TestWatcherCapacity(c *check.C) {
	const gracePeriod = time.Second
	clock := clockwork.NewFakeClock()

	ctx := context.Background()
	b := NewCircularBuffer(
		BufferCapacity(1),
		BufferClock(clock),
		BacklogGracePeriod(gracePeriod),
	)
	defer b.Close()
	b.SetInit()

	w, err := b.NewWatcher(ctx, Watch{
		QueueSize: 1,
	})
	c.Assert(err, check.IsNil)
	defer w.Close()

	select {
	case e := <-w.Events():
		c.Assert(e.Type, check.Equals, types.OpInit)
	default:
		c.Fatalf("Expected immediate OpInit.")
	}

	// emit and then consume 10 events.  this is much larger than our queue size,
	// but should succeed since we consume within our grace period.
	for i := 0; i < 10; i++ {
		b.Emit(Event{Item: Item{Key: []byte{Separator}, ID: int64(i + 1)}})
	}
	for i := 0; i < 10; i++ {
		select {
		case e := <-w.Events():
			c.Assert(e.Item.ID, check.Equals, int64(i+1))
		default:
			c.Fatalf("Expected events to be immediately available")
		}
	}

	// advance further than grace period.
	clock.Advance(gracePeriod + time.Second)

	// emit another event, which will cause buffer to reevaluate the grace period.
	b.Emit(Event{Item: Item{Key: []byte{Separator}, ID: int64(11)}})

	// ensure that buffer did not close watcher, since previously created backlog
	// was drained within grace period.
	select {
	case <-w.Done():
		c.Fatalf("Watcher should not have backlog, but was closed anyway")
	default:
	}

	// create backlog again, and this time advance past grace period without draining it.
	for i := 0; i < 10; i++ {
		b.Emit(Event{Item: Item{Key: []byte{Separator}, ID: int64(i + 12)}})
	}
	clock.Advance(gracePeriod + time.Second)

	// emit another event, which will cause buffer to realize that watcher is past
	// its grace period.
	b.Emit(Event{Item: Item{Key: []byte{Separator}, ID: int64(22)}})

	select {
	case <-w.Done():
	default:
		c.Fatalf("buffer did not close watcher that was past grace period")
	}
}

// TestWatcherClose makes sure that closed watcher
// will be removed
func (s *BufferSuite) TestWatcherClose(c *check.C) {
	b := NewCircularBuffer(
		BufferCapacity(3),
	)
	defer b.Close()
	b.SetInit()

	w, err := b.NewWatcher(context.TODO(), Watch{})
	c.Assert(err, check.IsNil)

	select {
	case e := <-w.Events():
		c.Assert(e.Type, check.Equals, types.OpInit)
	case <-time.After(100 * time.Millisecond):
		c.Fatalf("Timeout waiting for event.")
	}

	c.Assert(b.watchers.Len(), check.Equals, 1)
	w.(*BufferWatcher).closeAndRemove(removeSync)
	c.Assert(b.watchers.Len(), check.Equals, 0)
}

// TestRemoveRedundantPrefixes removes redundant prefixes
func (s *BufferSuite) TestRemoveRedundantPrefixes(c *check.C) {
	type tc struct {
		in  [][]byte
		out [][]byte
	}
	tcs := []tc{
		{
			in:  [][]byte{},
			out: [][]byte{},
		},
		{
			in:  [][]byte{[]byte("/a")},
			out: [][]byte{[]byte("/a")},
		},
		{
			in:  [][]byte{[]byte("/a"), []byte("/")},
			out: [][]byte{[]byte("/")},
		},
		{
			in:  [][]byte{[]byte("/b"), []byte("/a")},
			out: [][]byte{[]byte("/a"), []byte("/b")},
		},
		{
			in:  [][]byte{[]byte("/a/b"), []byte("/a"), []byte("/a/b/c"), []byte("/d")},
			out: [][]byte{[]byte("/a"), []byte("/d")},
		},
	}
	for _, tc := range tcs {
		c.Assert(removeRedundantPrefixes(tc.in), check.DeepEquals, tc.out)
	}
}

// TestWatcherMulti makes sure that watcher
// with multiple matching prefixes will get an event only once
func (s *BufferSuite) TestWatcherMulti(c *check.C) {
	b := NewCircularBuffer(
		BufferCapacity(3),
	)
	defer b.Close()
	b.SetInit()

	w, err := b.NewWatcher(context.TODO(), Watch{Prefixes: [][]byte{[]byte("/a"), []byte("/a/b")}})
	c.Assert(err, check.IsNil)
	defer w.Close()

	select {
	case e := <-w.Events():
		c.Assert(e.Type, check.Equals, types.OpInit)
	case <-time.After(100 * time.Millisecond):
		c.Fatalf("Timeout waiting for event.")
	}

	b.Emit(Event{Item: Item{Key: []byte("/a/b/c"), ID: 1}})

	select {
	case e := <-w.Events():
		c.Assert(e.Item.ID, check.Equals, int64(1))
	case <-time.After(100 * time.Millisecond):
		c.Fatalf("Timeout waiting for event.")
	}

	c.Assert(len(w.Events()), check.Equals, 0)

}

// TestWatcherReset tests scenarios with watchers and buffer resets
func (s *BufferSuite) TestWatcherReset(c *check.C) {
	b := NewCircularBuffer(
		BufferCapacity(3),
	)
	defer b.Close()
	b.SetInit()

	w, err := b.NewWatcher(context.TODO(), Watch{})
	c.Assert(err, check.IsNil)
	defer w.Close()

	select {
	case e := <-w.Events():
		c.Assert(e.Type, check.Equals, types.OpInit)
	case <-time.After(100 * time.Millisecond):
		c.Fatalf("Timeout waiting for event.")
	}

	b.Emit(Event{Item: Item{Key: []byte{Separator}, ID: 1}})
	b.Clear()

	// make sure watcher has been closed
	select {
	case <-w.Done():
	case <-time.After(100 * time.Millisecond):
		c.Fatalf("Timeout waiting for close event.")
	}

	w2, err := b.NewWatcher(context.TODO(), Watch{})
	c.Assert(err, check.IsNil)
	defer w2.Close()

	select {
	case e := <-w2.Events():
		c.Assert(e.Type, check.Equals, types.OpInit)
	case <-time.After(100 * time.Millisecond):
		c.Fatalf("Timeout waiting for event.")
	}

	b.Emit(Event{Item: Item{Key: []byte{Separator}, ID: 2}})

	select {
	case e := <-w2.Events():
		c.Assert(e.Item.ID, check.Equals, int64(2))
	case <-time.After(100 * time.Millisecond):
		c.Fatalf("Timeout waiting for event.")
	}
}

// TestWatcherTree tests buffer watcher tree
func (s *BufferSuite) TestWatcherTree(c *check.C) {
	t := newWatcherTree()
	c.Assert(t.rm(nil), check.Equals, false)

	w1 := &BufferWatcher{Watch: Watch{Prefixes: [][]byte{[]byte("/a"), []byte("/a/a1"), []byte("/c")}}}
	c.Assert(t.rm(w1), check.Equals, false)

	w2 := &BufferWatcher{Watch: Watch{Prefixes: [][]byte{[]byte("/a")}}}

	t.add(w1)
	t.add(w2)

	var out []*BufferWatcher
	t.walk(func(w *BufferWatcher) {
		out = append(out, w)
	})
	c.Assert(out, check.HasLen, 4)

	var matched []*BufferWatcher
	t.walkPath("/c", func(w *BufferWatcher) {
		matched = append(matched, w)
	})
	c.Assert(matched, check.HasLen, 1)
	c.Assert(matched[0], check.Equals, w1)

	matched = nil
	t.walkPath("/a", func(w *BufferWatcher) {
		matched = append(matched, w)
	})
	c.Assert(matched, check.HasLen, 2)
	c.Assert(matched[0], check.Equals, w1)
	c.Assert(matched[1], check.Equals, w2)

	c.Assert(t.rm(w1), check.Equals, true)
	c.Assert(t.rm(w1), check.Equals, false)

	matched = nil
	t.walkPath("/a", func(w *BufferWatcher) {
		matched = append(matched, w)
	})
	c.Assert(matched, check.HasLen, 1)
	c.Assert(matched[0], check.Equals, w2)

	c.Assert(t.rm(w2), check.Equals, true)
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
