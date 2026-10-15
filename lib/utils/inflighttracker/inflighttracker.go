/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package inflighttracker

import (
	"context"
	"sync"

	"github.com/gravitational/trace"
)

// Tracker counts how much work is currently in flight.
type Tracker struct {
	mu        sync.Mutex
	closed    bool
	count     int
	drainedCh chan struct{}
}

// Add adds one to the count of the number of items in flight.
// Every successful `Add()` needs a matching `Done()`.
// A return value of true indicates success, and false indicates a failure.
// Add will only fail if the tracker is closed.
func (t *Tracker) Add() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return false
	}
	t.count++
	return true
}

// Done marks an item as finished and decreases the count by one.
// Every successful `Add()` needs a matching `Done()`.
func (t *Tracker) Done() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.count--
	if t.count < 0 {
		panic("inflighttracker: negative count")
	}
	t.signalLocked()
}

// Go executes a new goroutine, tracks it in the count, and marks it as done
// when it completes.
func (t *Tracker) Go(fn func()) bool {
	if !t.Add() {
		return false
	}
	go func() {
		defer t.Done()
		fn()
	}()
	return true
}

// Close stops Tracker from accepting new work.
func (t *Tracker) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	t.signalLocked()
}

// Wait closes the tracker and blocks until all work that is currently in flight
// is complete.
func (t *Tracker) Wait(ctx context.Context) error {
	t.Close()
	select {
	case <-t.drained():
		return nil
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}

func (t *Tracker) drained() <-chan struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.drainedLocked()
}

func (t *Tracker) drainedLocked() chan struct{} {
	if t.drainedCh == nil {
		t.drainedCh = make(chan struct{})
	}
	return t.drainedCh
}

func (t *Tracker) signalLocked() {
	if !t.closed || t.count != 0 {
		return
	}
	select {
	case <-t.drainedLocked():
	default:
		close(t.drainedCh)
	}
}
