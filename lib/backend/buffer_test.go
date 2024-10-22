/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package backend

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// TestWatcherSimple tests scenarios with watchers
func TestWatcherSimple(t *testing.T) {
	ctx := context.Background()
	b := NewCircularBuffer(
		BufferCapacity(3),
	)
	defer b.Close()
	b.SetInit()

	w, err := b.NewWatcher(ctx, Watch{})
	require.NoError(t, err)
	defer w.Close()

	select {
	case e := <-w.Events():
		require.Equal(t, types.OpInit, e.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("Timeout waiting for event.")
	}

	b.Emit(Event{Item: Item{Key: NewKey("1")}})

	select {
	case e := <-w.Events():
		require.Equal(t, NewKey("1"), e.Item.Key)
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("Timeout waiting for event.")
	}

	b.Close()
	b.Emit(Event{Item: Item{Key: NewKey("2")}})

	select {
	case <-w.Done():
		// expected
	case <-w.Events():
		t.Fatalf("unexpected event")
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("Timeout waiting for event.")
	}
}

// TestWatcherCapacity checks various watcher capacity scenarios
func TestWatcherCapacity(t *testing.T) {
	const gracePeriod = time.Second
	clock := clockwork.NewFakeClock()

	ctx := context.Background()
	b := NewCircularBuffer(
		BufferCapacity(1),
		BufferClock(clock),
		BacklogGracePeriod(gracePeriod),
		CreationGracePeriod(time.Nanosecond),
	)
	defer b.Close()
	b.SetInit()

	w, err := b.NewWatcher(ctx, Watch{
		QueueSize: 1,
	})
	require.NoError(t, err)
	defer w.Close()

	select {
	case e := <-w.Events():
		require.Equal(t, types.OpInit, e.Type)
	default:
		t.Fatalf("Expected immediate OpInit.")
	}

	// emit and then consume 10 events.  this is much larger than our queue size,
	// but should succeed since we consume within our grace period.
	for i := 0; i < 10; i++ {
		b.Emit(Event{Item: Item{Key: NewKey(strconv.Itoa(i + 1))}})
	}
	for i := 0; i < 10; i++ {
		select {
		case e := <-w.Events():
			require.Equal(t, string(Separator)+strconv.Itoa(i+1), e.Item.Key.String())
		default:
			t.Fatalf("Expected events to be immediately available")
		}
	}

	// advance further than grace period.
	clock.Advance(gracePeriod + time.Second)

	// emit another event, which will cause buffer to reevaluate the grace period.
	b.Emit(Event{Item: Item{Key: NewKey("11")}})

	// ensure that buffer did not close watcher, since previously created backlog
	// was drained within grace period.
	select {
	case <-w.Done():
		t.Fatalf("Watcher should not have backlog, but was closed anyway")
	default:
	}

	// create backlog again, and this time advance past grace period without draining it.
	for i := 0; i < 10; i++ {
		b.Emit(Event{Item: Item{Key: NewKey(strconv.Itoa(i + 12))}})
	}
	clock.Advance(gracePeriod + time.Second)

	// emit another event, which will cause buffer to realize that watcher is past
	// its grace period.
	b.Emit(Event{Item: Item{Key: NewKey("22")}})

	select {
	case <-w.Done():
	default:
		t.Fatalf("buffer did not close watcher that was past grace period")
	}
}

func TestWatcherCreationGracePeriod(t *testing.T) {
	const backlogGracePeriod = time.Second
	const creationGracePeriod = backlogGracePeriod * 3
	const queueSize = 1
	clock := clockwork.NewFakeClock()

	ctx := context.Background()
	b := NewCircularBuffer(
		BufferCapacity(1),
		BufferClock(clock),
		BacklogGracePeriod(backlogGracePeriod),
		CreationGracePeriod(creationGracePeriod),
	)
	defer b.Close()
	b.SetInit()

	w, err := b.NewWatcher(ctx, Watch{
		QueueSize: queueSize,
	})
	require.NoError(t, err)
	defer w.Close()

	select {
	case e := <-w.Events():
		require.Equal(t, types.OpInit, e.Type)
	default:
		t.Fatalf("Expected immediate OpInit.")
	}

	// emit enough events to create a backlog
	for i := 0; i < queueSize*2; i++ {
		b.Emit(Event{Item: Item{Key: NewKey("")}})
	}

	select {
	case <-w.Done():
		t.Fatal("watcher closed unexpectedly")
	default:
	}

	// sanity-check
	require.Greater(t, creationGracePeriod, backlogGracePeriod*2)

	// advance well past the backlog grace period, but not past the creation grace period
	clock.Advance(backlogGracePeriod * 2)

	b.Emit(Event{Item: Item{Key: NewKey("")}})

	select {
	case <-w.Done():
		t.Fatal("watcher closed unexpectedly")
	default:
	}

	// advance well past creation grace period
	clock.Advance(creationGracePeriod)

	b.Emit(Event{Item: Item{Key: NewKey("")}})
	select {
	case <-w.Done():
	default:
		t.Fatal("watcher did not close after creation grace period exceeded")
	}
}

// TestWatcherClose makes sure that closed watcher
// will be removed
func TestWatcherClose(t *testing.T) {
	ctx := context.Background()
	b := NewCircularBuffer(
		BufferCapacity(3),
	)
	defer b.Close()
	b.SetInit()

	w, err := b.NewWatcher(ctx, Watch{})
	require.NoError(t, err)

	select {
	case e := <-w.Events():
		require.Equal(t, types.OpInit, e.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("Timeout waiting for event.")
	}

	require.Equal(t, 1, b.watchers.Len())
	w.(*BufferWatcher).closeAndRemove(removeSync)
	require.Equal(t, 0, b.watchers.Len())
}

// TestRemoveRedundantPrefixes removes redundant prefixes
func TestRemoveRedundantPrefixes(t *testing.T) {
	type tc struct {
		in  []Key
		out []Key
	}
	tcs := []tc{
		{
			in:  []Key{},
			out: []Key{},
		},
		{
			in:  []Key{NewKey("a")},
			out: []Key{NewKey("a")},
		},
		{
			in:  []Key{NewKey("a"), NewKey("")},
			out: []Key{NewKey("")},
		},
		{
			in:  []Key{NewKey("b"), NewKey("a")},
			out: []Key{NewKey("a"), NewKey("b")},
		},
		{
			in:  []Key{NewKey("a", "b"), NewKey("a"), NewKey("a", "b", "c"), NewKey("d")},
			out: []Key{NewKey("a"), NewKey("d")},
		},
	}
	for _, tc := range tcs {
		require.Empty(t, cmp.Diff(RemoveRedundantPrefixes(tc.in), tc.out, cmp.AllowUnexported(Key{})))
	}
}

// TestWatcherMulti makes sure that watcher
// with multiple matching prefixes will get an event only once
func TestWatcherMulti(t *testing.T) {
	ctx := context.Background()
	b := NewCircularBuffer(
		BufferCapacity(3),
	)
	defer b.Close()
	b.SetInit()

	w, err := b.NewWatcher(ctx, Watch{Prefixes: []Key{NewKey("a"), NewKey("a", "b")}})
	require.NoError(t, err)
	defer w.Close()

	select {
	case e := <-w.Events():
		require.Equal(t, types.OpInit, e.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("Timeout waiting for event.")
	}

	b.Emit(Event{Item: Item{Key: NewKey("a", "b", "c")}})

	select {
	case e := <-w.Events():
		require.Equal(t, NewKey("a", "b", "c"), e.Item.Key)
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("Timeout waiting for event.")
	}

	require.Empty(t, w.Events())
}

// TestWatcherReset tests scenarios with watchers and buffer resets
func TestWatcherReset(t *testing.T) {
	ctx := context.Background()
	b := NewCircularBuffer(
		BufferCapacity(3),
	)
	defer b.Close()
	b.SetInit()

	w, err := b.NewWatcher(ctx, Watch{})
	require.NoError(t, err)
	defer w.Close()

	select {
	case e := <-w.Events():
		require.Equal(t, types.OpInit, e.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("Timeout waiting for event.")
	}

	b.Emit(Event{Item: Item{Key: NewKey("1")}})
	b.Clear()

	// make sure watcher has been closed
	select {
	case <-w.Done():
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("Timeout waiting for close event.")
	}

	w2, err := b.NewWatcher(ctx, Watch{})
	require.NoError(t, err)
	defer w2.Close()

	select {
	case e := <-w2.Events():
		require.Equal(t, types.OpInit, e.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("Timeout waiting for event.")
	}

	b.Emit(Event{Item: Item{Key: NewKey("2")}})

	select {
	case e := <-w2.Events():
		require.Equal(t, NewKey("2"), e.Item.Key)
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("Timeout waiting for event.")
	}
}

// TestWatcherTree tests buffer watcher tree
func TestWatcherTree(t *testing.T) {
	wt := newWatcherTree()
	require.False(t, wt.rm(nil))

	w1 := &BufferWatcher{Watch: Watch{Prefixes: []Key{NewKey("a"), NewKey("a", "a1"), NewKey("c")}}}
	require.False(t, wt.rm(w1))

	w2 := &BufferWatcher{Watch: Watch{Prefixes: []Key{NewKey("a")}}}

	wt.add(w1)
	wt.add(w2)

	var out []*BufferWatcher
	wt.walk(func(w *BufferWatcher) {
		out = append(out, w)
	})
	require.Len(t, out, 4)

	var matched []*BufferWatcher
	wt.walkPath("/c", func(w *BufferWatcher) {
		matched = append(matched, w)
	})
	require.Len(t, matched, 1)
	require.Equal(t, matched[0], w1)

	matched = nil
	wt.walkPath("/a", func(w *BufferWatcher) {
		matched = append(matched, w)
	})
	require.Len(t, matched, 2)
	require.Equal(t, matched[0], w1)
	require.Equal(t, matched[1], w2)

	require.True(t, wt.rm(w1))
	require.False(t, wt.rm(w1))

	matched = nil
	wt.walkPath("/a", func(w *BufferWatcher) {
		matched = append(matched, w)
	})
	require.Len(t, matched, 1)
	require.Equal(t, matched[0], w2)

	require.True(t, wt.rm(w2))
}
