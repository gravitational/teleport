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

	b.Emit(Event{Item: Item{Key: []byte{Separator}, ID: 1}})

	select {
	case e := <-w.Events():
		require.Equal(t, int64(1), e.Item.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("Timeout waiting for event.")
	}

	b.Close()
	b.Emit(Event{Item: Item{ID: 2}})

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
		b.Emit(Event{Item: Item{Key: []byte{Separator}, ID: int64(i + 1)}})
	}
	for i := 0; i < 10; i++ {
		select {
		case e := <-w.Events():
			require.Equal(t, e.Item.ID, int64(i+1))
		default:
			t.Fatalf("Expected events to be immediately available")
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
		t.Fatalf("Watcher should not have backlog, but was closed anyway")
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
		t.Fatalf("buffer did not close watcher that was past grace period")
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
		require.Empty(t, cmp.Diff(RemoveRedundantPrefixes(tc.in), tc.out))
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

	w, err := b.NewWatcher(ctx, Watch{Prefixes: [][]byte{[]byte("/a"), []byte("/a/b")}})
	require.NoError(t, err)
	defer w.Close()

	select {
	case e := <-w.Events():
		require.Equal(t, types.OpInit, e.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("Timeout waiting for event.")
	}

	b.Emit(Event{Item: Item{Key: []byte("/a/b/c"), ID: 1}})

	select {
	case e := <-w.Events():
		require.Equal(t, int64(1), e.Item.ID)
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

	b.Emit(Event{Item: Item{Key: []byte{Separator}, ID: 1}})
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

	b.Emit(Event{Item: Item{Key: []byte{Separator}, ID: 2}})

	select {
	case e := <-w2.Events():
		require.Equal(t, int64(2), e.Item.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("Timeout waiting for event.")
	}
}

// TestWatcherTree tests buffer watcher tree
func TestWatcherTree(t *testing.T) {
	wt := newWatcherTree()
	require.False(t, wt.rm(nil))

	w1 := &BufferWatcher{Watch: Watch{Prefixes: [][]byte{[]byte("/a"), []byte("/a/a1"), []byte("/c")}}}
	require.False(t, wt.rm(w1))

	w2 := &BufferWatcher{Watch: Watch{Prefixes: [][]byte{[]byte("/a")}}}

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
