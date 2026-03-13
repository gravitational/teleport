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
	"math"
	"sync"
	"sync/atomic"

	debugpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/debug/v1"
)

const (
	// subscriberBufferSize is the buffer size of each subscriber's channel.
	subscriberBufferSize = 256
	// maxSubscribers is the maximum number of concurrent subscribers.
	maxSubscribers = 10
	// maxLevel is a sentinel meaning "no subscribers".
	maxLevel = slog.Level(math.MaxInt32)
)

// subscriber holds a channel and the minimum level the subscriber wants.
type subscriber struct {
	ch    chan *debugpb.LogEntry
	level slog.Level
}

// LogBroadcaster manages subscriber channels for log streaming with
// per-subscriber level filtering. It is safe for concurrent use.
// When no subscribers are active, the overhead in BroadcastHandler.Enabled
// is a single atomic load.
type LogBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan *debugpb.LogEntry]*subscriber
	// minLevel is the minimum level across all active subscribers.
	// Updated under write lock, read atomically in Enabled.
	minLevel atomic.Int64
}

// NewLogBroadcaster creates a new LogBroadcaster.
func NewLogBroadcaster() *LogBroadcaster {
	b := &LogBroadcaster{
		subscribers: make(map[chan *debugpb.LogEntry]*subscriber),
	}
	b.minLevel.Store(int64(maxLevel))
	return b
}

// MinLevel returns the minimum level across all subscribers.
// Returns maxLevel (a high sentinel) when no subscribers exist.
func (b *LogBroadcaster) MinLevel() slog.Level {
	return slog.Level(b.minLevel.Load())
}

// Subscribe creates a subscription at the given minimum level.
// Returns nil if the maximum subscriber count has been reached.
func (b *LogBroadcaster) Subscribe(level slog.Level) chan *debugpb.LogEntry {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.subscribers) >= maxSubscribers {
		return nil
	}

	ch := make(chan *debugpb.LogEntry, subscriberBufferSize)
	b.subscribers[ch] = &subscriber{ch: ch, level: level}
	b.recalcMinLevel()
	return ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (b *LogBroadcaster) Unsubscribe(ch chan *debugpb.LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.subscribers[ch]; ok {
		delete(b.subscribers, ch)
		close(ch)
		b.recalcMinLevel()
	}
}

// Broadcast sends a log entry to all subscribers whose level
// threshold is at or below the given record level.
func (b *LogBroadcaster) Broadcast(entry *debugpb.LogEntry, level slog.Level) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, sub := range b.subscribers {
		if level < sub.level {
			continue
		}
		select {
		case sub.ch <- entry:
		default:
			// Drop on slow consumer.
		}
	}
}

// recalcMinLevel must be called with b.mu held (write lock).
func (b *LogBroadcaster) recalcMinLevel() {
	if len(b.subscribers) == 0 {
		b.minLevel.Store(int64(maxLevel))
		return
	}
	min := maxLevel
	for _, sub := range b.subscribers {
		if sub.level < min {
			min = sub.level
		}
	}
	b.minLevel.Store(int64(min))
}
