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
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
)

func TestBroadcastHandler_Enabled(t *testing.T) {
	t.Parallel()

	t.Run("inner enabled, no subscribers", func(t *testing.T) {
		inner := &levelHandler{level: slog.LevelInfo}
		b := NewLogBroadcaster()
		h := NewBroadcastHandler(inner, b)

		assert.True(t, h.Enabled(context.Background(), slog.LevelInfo))
		assert.True(t, h.Enabled(context.Background(), slog.LevelError))
		assert.False(t, h.Enabled(context.Background(), slog.LevelDebug))
	})

	t.Run("inner disabled, subscriber wants debug", func(t *testing.T) {
		inner := &levelHandler{level: slog.LevelError}
		b := NewLogBroadcaster()
		h := NewBroadcastHandler(inner, b)

		ch := b.Subscribe(slog.LevelDebug)
		require.NotNil(t, ch)

		// Inner only wants error, but subscriber wants debug.
		assert.True(t, h.Enabled(context.Background(), slog.LevelDebug))
		assert.True(t, h.Enabled(context.Background(), slog.LevelInfo))
		assert.True(t, h.Enabled(context.Background(), slog.LevelError))
	})

	t.Run("no subscribers, inner disabled", func(t *testing.T) {
		inner := &levelHandler{level: slog.LevelError}
		b := NewLogBroadcaster()
		h := NewBroadcastHandler(inner, b)

		assert.False(t, h.Enabled(context.Background(), slog.LevelInfo))
	})
}

func TestBroadcastHandler_Handle(t *testing.T) {
	t.Parallel()

	t.Run("broadcasts LogEntry to subscriber", func(t *testing.T) {
		inner := &levelHandler{level: slog.LevelInfo}
		b := NewLogBroadcaster()
		h := NewBroadcastHandler(inner, b)

		ch := b.Subscribe(slog.LevelInfo)
		require.NotNil(t, ch)

		r := slog.NewRecord(time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC), slog.LevelInfo, "test message", 0)
		r.AddAttrs(slog.String("key", "value"))

		err := h.Handle(context.Background(), r)
		require.NoError(t, err)

		select {
		case entry := <-ch:
			assert.Equal(t, "INFO", entry.Level)
			assert.Equal(t, "test message", entry.Message)
			assert.Equal(t, "value", entry.Attributes["key"])
			assert.Equal(t, "2025-01-15T10:30:00Z", entry.Timestamp.AsTime().Format(time.RFC3339Nano))
		default:
			t.Fatal("expected data on subscriber channel")
		}
	})

	t.Run("does not broadcast below subscriber level", func(t *testing.T) {
		inner := &levelHandler{level: slog.LevelDebug}
		b := NewLogBroadcaster()
		h := NewBroadcastHandler(inner, b)

		ch := b.Subscribe(slog.LevelWarn)
		require.NotNil(t, ch)

		r := slog.NewRecord(time.Now(), slog.LevelInfo, "low level", 0)
		err := h.Handle(context.Background(), r)
		require.NoError(t, err)

		select {
		case <-ch:
			t.Fatal("should not have received data")
		default:
		}
	})

	t.Run("forwards to inner even without subscribers", func(t *testing.T) {
		inner := &levelHandler{level: slog.LevelInfo}
		b := NewLogBroadcaster()
		h := NewBroadcastHandler(inner, b)

		r := slog.NewRecord(time.Now(), slog.LevelInfo, "hello", 0)
		err := h.Handle(context.Background(), r)
		require.NoError(t, err)
		assert.Equal(t, 1, inner.handleCount)
	})

	t.Run("does not forward to inner below inner level", func(t *testing.T) {
		inner := &levelHandler{level: slog.LevelError}
		b := NewLogBroadcaster()
		h := NewBroadcastHandler(inner, b)

		ch := b.Subscribe(slog.LevelDebug)
		require.NotNil(t, ch)

		r := slog.NewRecord(time.Now(), slog.LevelInfo, "low for inner", 0)
		err := h.Handle(context.Background(), r)
		require.NoError(t, err)
		assert.Equal(t, 0, inner.handleCount)

		// But subscriber should have received it.
		select {
		case <-ch:
		default:
			t.Fatal("expected data on subscriber channel")
		}
	})
}

func TestBroadcastHandler_ToLogEntry(t *testing.T) {
	t.Parallel()

	t.Run("level strings", func(t *testing.T) {
		tests := []struct {
			level    slog.Level
			expected string
		}{
			{TraceLevel, TraceLevelText},
			{slog.LevelDebug, "DEBUG"},
			{slog.LevelInfo, "INFO"},
			{slog.LevelWarn, "WARN"},
			{slog.LevelError, "ERROR"},
		}

		for _, tt := range tests {
			inner := &levelHandler{level: TraceLevel}
			b := NewLogBroadcaster()
			h := NewBroadcastHandler(inner, b)

			ch := b.Subscribe(TraceLevel)
			require.NotNil(t, ch)

			r := slog.NewRecord(time.Now(), tt.level, "msg", 0)
			require.NoError(t, h.Handle(context.Background(), r))

			entry := <-ch
			assert.Equal(t, tt.expected, entry.Level, "level %v", tt.level)
		}
	})

	t.Run("component key set on entry", func(t *testing.T) {
		inner := &levelHandler{level: slog.LevelInfo}
		b := NewLogBroadcaster()
		h := NewBroadcastHandler(inner, b)

		ch := b.Subscribe(slog.LevelInfo)
		require.NotNil(t, ch)

		r := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
		r.AddAttrs(slog.String(teleport.ComponentKey, "AUTH"))

		require.NoError(t, h.Handle(context.Background(), r))

		entry := <-ch
		assert.Equal(t, "AUTH", entry.Component)
		// The component key should NOT appear in the attributes map.
		_, hasRawKey := entry.Attributes[teleport.ComponentKey]
		assert.False(t, hasRawKey)
	})

	t.Run("various value types", func(t *testing.T) {
		inner := &levelHandler{level: slog.LevelInfo}
		b := NewLogBroadcaster()
		h := NewBroadcastHandler(inner, b)

		ch := b.Subscribe(slog.LevelInfo)
		require.NotNil(t, ch)

		r := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
		r.AddAttrs(
			slog.Int64("count", 42),
			slog.Bool("ok", true),
			slog.Duration("elapsed", 5*time.Second),
			slog.Float64("ratio", 3.14),
		)

		require.NoError(t, h.Handle(context.Background(), r))

		entry := <-ch
		assert.Equal(t, "42", entry.Attributes["count"])
		assert.Equal(t, "true", entry.Attributes["ok"])
		assert.Equal(t, "5s", entry.Attributes["elapsed"])
		assert.Equal(t, "3.14", entry.Attributes["ratio"])
	})
}

func TestBroadcastHandler_WithAttrs(t *testing.T) {
	t.Parallel()

	t.Run("preAttrs appear in output", func(t *testing.T) {
		inner := &levelHandler{level: slog.LevelInfo}
		b := NewLogBroadcaster()
		h := NewBroadcastHandler(inner, b)

		h2 := h.WithAttrs([]slog.Attr{slog.String("service", "auth")})

		ch := b.Subscribe(slog.LevelInfo)
		require.NotNil(t, ch)

		r := slog.NewRecord(time.Now(), slog.LevelInfo, "hello", 0)
		require.NoError(t, h2.Handle(context.Background(), r))

		entry := <-ch
		assert.Equal(t, "auth", entry.Attributes["service"])
	})

	t.Run("original handler unchanged", func(t *testing.T) {
		inner := &levelHandler{level: slog.LevelInfo}
		b := NewLogBroadcaster()
		h := NewBroadcastHandler(inner, b)

		_ = h.WithAttrs([]slog.Attr{slog.String("extra", "yes")})

		ch := b.Subscribe(slog.LevelInfo)
		require.NotNil(t, ch)

		r := slog.NewRecord(time.Now(), slog.LevelInfo, "hello", 0)
		require.NoError(t, h.Handle(context.Background(), r))

		entry := <-ch
		_, hasExtra := entry.Attributes["extra"]
		assert.False(t, hasExtra)
	})

	t.Run("empty attrs returns same handler", func(t *testing.T) {
		inner := &levelHandler{level: slog.LevelInfo}
		b := NewLogBroadcaster()
		h := NewBroadcastHandler(inner, b)

		h2 := h.WithAttrs(nil)
		assert.Same(t, h, h2)
	})
}

func TestBroadcastHandler_WithGroup(t *testing.T) {
	t.Parallel()

	t.Run("attrs dot-prefixed under group", func(t *testing.T) {
		inner := &levelHandler{level: slog.LevelInfo}
		b := NewLogBroadcaster()
		h := NewBroadcastHandler(inner, b)

		h2 := h.WithGroup("request")

		ch := b.Subscribe(slog.LevelInfo)
		require.NotNil(t, ch)

		r := slog.NewRecord(time.Now(), slog.LevelInfo, "hello", 0)
		r.AddAttrs(slog.String("method", "GET"))

		require.NoError(t, h2.Handle(context.Background(), r))

		entry := <-ch
		assert.Equal(t, "GET", entry.Attributes["request.method"])
	})

	t.Run("preAttrs tagged with group at time of WithAttrs", func(t *testing.T) {
		inner := &levelHandler{level: slog.LevelInfo}
		b := NewLogBroadcaster()
		h := NewBroadcastHandler(inner, b)

		// Add group, then attrs.
		h2 := h.WithGroup("ctx").WithAttrs([]slog.Attr{slog.String("user", "alice")})

		ch := b.Subscribe(slog.LevelInfo)
		require.NotNil(t, ch)

		r := slog.NewRecord(time.Now(), slog.LevelInfo, "hello", 0)
		require.NoError(t, h2.Handle(context.Background(), r))

		entry := <-ch
		assert.Equal(t, "alice", entry.Attributes["ctx.user"])
	})

	t.Run("empty group name returns same handler", func(t *testing.T) {
		inner := &levelHandler{level: slog.LevelInfo}
		b := NewLogBroadcaster()
		h := NewBroadcastHandler(inner, b)

		h2 := h.WithGroup("")
		assert.Same(t, h, h2)
	})
}

// levelHandler is a minimal slog.Handler for testing that tracks calls.
type levelHandler struct {
	level       slog.Level
	handleCount int
}

func (h *levelHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *levelHandler) Handle(_ context.Context, _ slog.Record) error {
	h.handleCount++
	return nil
}

func (h *levelHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

func (h *levelHandler) WithGroup(_ string) slog.Handler {
	return h
}
