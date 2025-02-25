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

package fanoutbuffer

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jonboulle/clockwork"
)

// ErrGracePeriodExceeded is an error returned by Cursor.Read indicating that the cursor fell
// too far behind. Observing this error indicates that either the reader is too slow, or the
// buffer has been configured with insufficient capacity/backlog for the usecase.
var ErrGracePeriodExceeded = errors.New("failed to process fanout buffer backlog within grace period")

// ErrUseOfClosedCursor is an error indicating that Cursor.Read was called after the cursor had
// either been explicitly closed, or had previously returned an error.
var ErrUseOfClosedCursor = errors.New("use of closed fanout buffer cursor (this is a bug)")

// ErrBufferClosed is an error indicating that the event buffer as a whole has been closed.
var ErrBufferClosed = errors.New("fanout buffer closed")

// Config holds all configuration parameters for the fanout buffer. All parameters are optional.
type Config struct {
	// Capacity is the capacity to allocate for the main circular buffer. Capacity should be selected s.t. cursors rarely
	// fall behind capacity during normal operation.
	Capacity uint64

	// GracePeriod is the amount of time a backlog (beyond the specified capacity) will be allowed to persist. Longer grace
	// periods give cursors more time to catch up in the event of spikes, at the cost of higher potential memory usage and
	// longer waits before unhealthy cursors are ejected.
	GracePeriod time.Duration

	// Clock is used to override default time-behaviors in tests.
	Clock clockwork.Clock
}

// SetDefaults sets default config parameters.
func (c *Config) SetDefaults() {
	if c.Capacity == 0 {
		c.Capacity = 512
	}

	if c.GracePeriod == 0 {
		c.GracePeriod = 30 * time.Second
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
}

// Buffer is a circular buffer that keeps track of how many cursors exist, and how many have seen
// each item, so that it knows when items can be cleared. If one or more cursors fall behind, the items
// they have yet to see go into a temporary backlog of infinite size. If the backlog persists for greater
// than the allowed grace period, it is cleared and all cursors still in the backlog fail on their next
// attempt to read.
type Buffer[T any] struct {
	rw sync.RWMutex

	// cfg holds buffer configuration.
	cfg Config

	// notify is closed and replaced each time an item is added to wake any cursors
	// that are fully caught up.
	notify chan struct{}

	// closed indicates that the buffer has been closed.
	closed bool

	// cursors is the total number of cursors that exist. used to determine the number of cursors expected
	// to observe each item.
	cursors uint64

	// pos is the absolute position of the oldest element in the ring, meaning that pos%Capacity is the index
	// of that element.
	pos uint64

	// len is the total number of elements in the ring (i.e. the index of the newest element is at `(pos + len - 1) % Capacity`).
	len uint64

	// ring is the fixed-size circular buffer that serves as our primary storage location.
	ring []entry[T]

	// overflow is a variable length buffer of items that still need to be observed.
	overflow []overflowEntry[T]
}

// entry represents an item in a buffer
type entry[T any] struct {
	item T
	// wait is the number of cursors that still need to observe this item. each cursor
	// decrements this value when it observes the item. items with wait==0 are cleaned
	// up. Note that this atomic is only safe to decrement while read lock is held. Entries
	// may be moved while write lock is held.
	wait atomic.Uint64
}

type overflowEntry[T any] struct {
	entry[T]
	expires time.Time
}

// NewBuffer creates a new fanout buffer instance.
func NewBuffer[T any](cfg Config) *Buffer[T] {
	cfg.SetDefaults()
	return &Buffer[T]{
		cfg:    cfg,
		notify: make(chan struct{}),
		ring:   make([]entry[T], cfg.Capacity),
	}
}

// Close permanently closes the fanout buffer. Note that all cursors are terminated immediately
// and may therefore miss items that had not been read at the time Close is called.
func (b *Buffer[T]) Close() {
	b.write(func() {
		b.closed = true
		close(b.notify)
	})
}

// NewCursor gets a new cursor into the buffer. Cursors *must* be closed as soon as they are
// no longer being read from. Stale cursors may cause performance degredation.
func (b *Buffer[T]) NewCursor() *Cursor[T] {
	var c uint64
	b.write(func() {
		if b.closed {
			return
		}
		c = b.pos + b.len
		b.cursors++
	})

	cursor := &Cursor[T]{
		buf:    b,
		cursor: c,
	}
	runtime.SetFinalizer(cursor, finalizeCursor[T])
	return cursor
}

// Append appends to the buffer and wakes all dormant cursors.
func (b *Buffer[T]) Append(items ...T) {
	var notify chan struct{}

	b.write(func() {
		if b.closed {
			panic("Append called on closed fanout buffer")
		}

		// free up slots that are no longer needed
		b.cleanupSlots()

		for _, item := range items {
			// ensure that there is at least one free slot in ring
			b.ensureSlot()

			// insert item into next free slot in ring
			idx := int((b.pos + b.len) % b.cfg.Capacity)
			b.ring[idx].item = item
			b.ring[idx].wait.Store(b.cursors)
			b.len++
		}

		notify = b.notify
		b.notify = make(chan struct{})
	})

	// notify all waiting cursors. we do this outside of the
	// lock to (theoretically) reduce potential contention.
	close(notify)
}

// cleanupSlots frees up enries that have been seen by all cursors and clears the backlog if
// it has exceeded its grace period.
func (b *Buffer[T]) cleanupSlots() {
	// trim items from overflow that have been seen by all cursors or are past their expiry
	now := b.cfg.Clock.Now()
	var clearOverflowTo int
	for i := 0; i < len(b.overflow); i++ {
		clearOverflowTo = i
		if b.overflow[i].wait.Load() > 0 && b.overflow[i].expires.After(now) {
			break
		}
	}

	clear(b.overflow[:clearOverflowTo])
	b.overflow = b.overflow[clearOverflowTo:]

	// clear overflow start state if overflow is empty
	if len(b.overflow) == 0 {
		b.overflow = nil
	}

	// clear items from ring that have been seen by all cursors
	for b.len > 0 && b.ring[int(b.pos%b.cfg.Capacity)].wait.Load() == 0 {
		b.ring[int(b.pos%b.cfg.Capacity)] = entry[T]{}
		b.pos++
		b.len--
	}
}

// ensure that there is at least one open slot in the ring. overflows items as needed.
func (b *Buffer[T]) ensureSlot() {
	if b.len < b.cfg.Capacity {
		// we have at least one free slot in ring, no need for overflow
		return
	}

	// copy oldest entry to overflow. note the that it would actually be safe to just append the
	// entire `entry` value rather than this append -> load -> store pattern that we follow
	// here, but 'go vet' doesn't understand our locking semantics so we need to avoid implicitly
	// copying the atomic, even when the write lock is held.
	b.overflow = append(b.overflow, overflowEntry[T]{
		entry: entry[T]{
			item: b.ring[int(b.pos%b.cfg.Capacity)].item,
		},
		expires: b.cfg.Clock.Now().Add(b.cfg.GracePeriod),
	})
	b.overflow[len(b.overflow)-1].wait.Store(b.ring[int(b.pos%b.cfg.Capacity)].wait.Load())

	// clear previous entry location.
	b.ring[int(b.pos%b.cfg.Capacity)] = entry[T]{}
	b.pos++
	b.len--
}

func (b *Buffer[T]) read(fn func()) {
	b.rw.RLock()
	defer b.rw.RUnlock()
	fn()
}

func (b *Buffer[T]) write(fn func()) {
	b.rw.Lock()
	defer b.rw.Unlock()
	fn()
}

// getEntry gets the entry for a given cursor value. if entry is nil, the cursor is up to date. if healthy is false,
// then the cursor is too behind and no longer valid. The entry pointer is only valid so long
// as the lock is held.
func (b *Buffer[T]) getEntry(cursor uint64) (e *entry[T], healthy bool) {
	if cursor >= b.pos+b.len {
		// cursor has seen all items
		return nil, true
	}

	if cursor >= b.pos {
		// cursor is between oldest and newest item
		return &b.ring[int(cursor%b.cfg.Capacity)], true
	}

	if off := b.pos - cursor; int(off) <= len(b.overflow) {
		// cursor is within the backlog
		return &b.overflow[len(b.overflow)-int(off)].entry, true
	}

	return nil, false
}

// Cursor is a cursor into a fanout buffer. Cursor's *must* be closed if not being actively read to avoid
// buffer performance degredation. Cursors are not intended for concurrent use (though they are "safe",
// concurrent calls may block longer than expected due to the lock being held across blocking reads).
type Cursor[T any] struct {
	buf    *Buffer[T]
	mu     sync.Mutex
	cursor uint64
	closed bool
}

// Read blocks until items become available and then reads them into the supplied buffer, returning the
// number of items that were read. Buffer size should be selected based on the expected throughput.
func (c *Cursor[T]) Read(ctx context.Context, out []T) (n int, err error) {
	return c.read(ctx, out, true)
}

// TryRead performs a non-blocking read. returns (0, nil) if no output is available.
func (c *Cursor[T]) TryRead(out []T) (n int, err error) {
	return c.read(context.Background(), out, false)
}

func (c *Cursor[T]) read(ctx context.Context, out []T, blocking bool) (n int, err error) {
	c.mu.Lock()
	defer func() {
		if err != nil {
			// note that we're a bit aggressive about closure here. in theory it would be
			// acceptable to leave a cursor in a usable state after context cancellation.
			// we opt to err on the side of caution here and close on all errors in order to
			// limit the chances that misuse results in a leaked cursor.
			c.closeLocked()
		}
		c.mu.Unlock()
	}()

	if len(out) == 0 {
		// panic on empty output buffer (consistent with how standard library treats reads with empty buffers)
		panic("empty buffer in Cursor.Read")
	}

	if c.closed {
		return n, ErrUseOfClosedCursor
	}

	for {
		var notify chan struct{}
		var healthy bool
		var closed bool
		c.buf.read(func() {
			if c.buf.closed {
				closed = true
				return
			}
			notify = c.buf.notify
			for i := range out {
				entry, h := c.buf.getEntry(c.cursor)
				healthy = h
				if entry == nil {
					return
				}
				n++
				c.cursor++
				// pointer to entry is only valid while lock is held,
				// so we decrement counter and extract item here.
				entry.wait.Add(^uint64(0))
				out[i] = entry.item
			}
		})

		if closed {
			// buffer has been closed
			return n, ErrBufferClosed
		}

		if !healthy {
			// fell too far behind
			return n, ErrGracePeriodExceeded
		}

		if n != 0 || !blocking {
			return n, nil
		}

		select {
		case <-notify:
		case <-ctx.Done():
			return n, ctx.Err()
		}
	}
}

func finalizeCursor[T any](cursor *Cursor[T]) {
	cursor.mu.Lock()
	defer cursor.mu.Unlock()
	if cursor.closed {
		return
	}

	cursor.closeLocked()
	slog.WarnContext(context.Background(), "Fanout buffer cursor was never closed. (this is a bug)")
}

// Close closes the cursor. Close is safe to double-call and should be called as soon as possible if
// the cursor is no longer in use.
func (c *Cursor[T]) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeLocked()
	return nil
}

func (c *Cursor[T]) closeLocked() {
	if c.closed {
		return
	}

	c.closed = true

	var cend uint64
	c.buf.write(func() {
		if c.buf.closed {
			return
		}
		cend = c.buf.pos + c.buf.len
		c.buf.cursors--
	})

	c.buf.read(func() {
		if c.buf.closed {
			return
		}
		// scan through all unread values and decrement their wait counters, since this
		// watcher will never read them.
		for cc := c.cursor; cc < cend; cc++ {
			entry, _ := c.buf.getEntry(cc)
			if entry != nil {
				// decrement entry's wait count
				entry.wait.Add(^uint64(0))
			}
		}
	})
}
