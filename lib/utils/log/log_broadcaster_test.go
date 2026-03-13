// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package log

import (
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	debugpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/debug/v1"
)

func TestLogBroadcaster_MinLevel(t *testing.T) {
	t.Parallel()

	t.Run("no subscribers returns maxLevel sentinel", func(t *testing.T) {
		b := NewLogBroadcaster()
		assert.Equal(t, maxLevel, b.MinLevel())
	})

	t.Run("tracks minimum across subscribers", func(t *testing.T) {
		b := NewLogBroadcaster()

		ch1 := b.Subscribe(slog.LevelWarn)
		require.NotNil(t, ch1)
		assert.Equal(t, slog.LevelWarn, b.MinLevel())

		ch2 := b.Subscribe(slog.LevelDebug)
		require.NotNil(t, ch2)
		assert.Equal(t, slog.LevelDebug, b.MinLevel())

		b.Unsubscribe(ch2)
		assert.Equal(t, slog.LevelWarn, b.MinLevel())

		b.Unsubscribe(ch1)
		assert.Equal(t, maxLevel, b.MinLevel())
	})
}

func TestLogBroadcaster_Subscribe(t *testing.T) {
	t.Parallel()

	t.Run("max subscribers", func(t *testing.T) {
		b := NewLogBroadcaster()
		channels := make([]chan *debugpb.LogEntry, 0, maxSubscribers)

		for range maxSubscribers {
			ch := b.Subscribe(slog.LevelInfo)
			require.NotNil(t, ch)
			channels = append(channels, ch)
		}

		// One more should fail.
		ch := b.Subscribe(slog.LevelInfo)
		assert.Nil(t, ch)

		// Free one, should work again.
		b.Unsubscribe(channels[0])
		ch = b.Subscribe(slog.LevelInfo)
		assert.NotNil(t, ch)
	})
}

func TestLogBroadcaster_Unsubscribe(t *testing.T) {
	t.Parallel()

	t.Run("closes channel", func(t *testing.T) {
		b := NewLogBroadcaster()
		ch := b.Subscribe(slog.LevelInfo)
		require.NotNil(t, ch)

		b.Unsubscribe(ch)
		_, ok := <-ch
		assert.False(t, ok, "channel should be closed")
	})

	t.Run("double unsubscribe is safe", func(t *testing.T) {
		b := NewLogBroadcaster()
		ch := b.Subscribe(slog.LevelInfo)
		require.NotNil(t, ch)

		b.Unsubscribe(ch)
		b.Unsubscribe(ch) // should not panic
	})
}

func TestLogBroadcaster_Broadcast(t *testing.T) {
	t.Parallel()

	t.Run("delivers to matching subscribers", func(t *testing.T) {
		b := NewLogBroadcaster()

		chDebug := b.Subscribe(slog.LevelDebug)
		chWarn := b.Subscribe(slog.LevelWarn)
		require.NotNil(t, chDebug)
		require.NotNil(t, chWarn)

		entry := &debugpb.LogEntry{Level: "info", Message: "test"}
		b.Broadcast(entry, slog.LevelInfo)

		// Debug subscriber wants debug+, should get info.
		select {
		case got := <-chDebug:
			assert.Equal(t, entry, got)
		default:
			t.Fatal("debug subscriber should have received data")
		}

		// Warn subscriber wants warn+, should not get info.
		select {
		case <-chWarn:
			t.Fatal("warn subscriber should not have received info-level data")
		default:
		}
	})

	t.Run("drops on slow consumer", func(t *testing.T) {
		b := NewLogBroadcaster()
		ch := b.Subscribe(slog.LevelInfo)
		require.NotNil(t, ch)

		// Fill the buffer.
		for range subscriberBufferSize {
			b.Broadcast(&debugpb.LogEntry{Message: "x"}, slog.LevelInfo)
		}

		// Next broadcast should not block.
		done := make(chan struct{})
		go func() {
			b.Broadcast(&debugpb.LogEntry{Message: "overflow"}, slog.LevelInfo)
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("Broadcast should not block on slow consumer")
		}
	})
}

func TestLogBroadcaster_ConcurrentSafety(t *testing.T) {
	t.Parallel()

	b := NewLogBroadcaster()
	const goroutines = 20
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			for range iterations {
				ch := b.Subscribe(slog.LevelInfo)
				if ch == nil {
					continue
				}
				b.Broadcast(&debugpb.LogEntry{Message: "data"}, slog.LevelInfo)
				b.Unsubscribe(ch)
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, maxLevel, b.MinLevel(), "all subscribers should be cleaned up")
}
