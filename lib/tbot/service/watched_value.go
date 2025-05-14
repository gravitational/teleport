/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
package service

import (
	"container/list"
	"context"
	"sync"
)

// WatchedValue wraps a value so that it can be shared between goroutines and
// consumers can "watch" for changes.
type WatchedValue[T comparable] struct {
	mu       sync.RWMutex
	val      T
	watchers *list.List
}

// NewWatchedValue returns a WatchedValue wrapping the given initial value.
func NewWatchedValue[T comparable](initialValue T) *WatchedValue[T] {
	return &WatchedValue[T]{
		val:      initialValue,
		watchers: list.New(),
	}
}

// Get the current value.
func (v *WatchedValue[T]) Get() T {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.val
}

// Set the value and notify any watchers. The changed return value indicates
// whether the value was changed or not.
func (v *WatchedValue[T]) Set(value T) (changed bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if value == v.val {
		return false
	}

	v.val = value

	for item := v.watchers.Front(); item != nil; item = item.Next() {
		item.Value.(*ValueWatcher[T]).newValue(value)
	}

	return true
}

// Watch returns the current value and a watcher on which you can call Wait to
// find out when the value changes. Callers are responsible for releasing the
// watcher's resources by calling Close.
func (v *WatchedValue[T]) Watch() (current T, watcher *ValueWatcher[T]) {
	v.mu.Lock()
	defer v.mu.Unlock()

	w := &ValueWatcher[T]{ch: make(chan struct{})}
	item := v.watchers.PushBack(w)
	w.closeFn = func() {
		v.mu.Lock()
		defer v.mu.Unlock()
		v.watchers.Remove(item)
	}

	return v.val, w
}

// ValueWatcher is returned from WatchedValue.Watch.
type ValueWatcher[T comparable] struct {
	mu      sync.Mutex
	val     T
	ch      chan struct{}
	closeFn func()
}

// Close releases the watcher's resources. Calling Wait on a closed watcher
// blocks forever.
func (w *ValueWatcher[T]) Close() { w.closeFn() }

// Ready returns a channel you can receive from to be notified when the value is
// next updated. The returned channel will be closed on-update, so you must call
// Ready again to wait for the next update rather than reusing the channel.
func (w *ValueWatcher[T]) Ready() <-chan struct{} {
	w.mu.Lock()
	defer w.mu.Unlock()

	ch := w.ch

	// If the channel has already been closed, we return it so that it unblocks
	// the caller immediately and replace it so that the next caller blocks until
	// there's a new value.
	select {
	case <-ch:
		w.ch = make(chan struct{})
	default:
	}

	return ch
}

// Wait for the value to change, or for the given context to be canceled or
// reach its deadline. It is not safe to be used by multiple concurrent callers,
// instead create a separate watcher for each.
//
// Note: Wait returns the *most recent value*, not a complete stream of every value.
func (w *ValueWatcher[T]) Wait(ctx context.Context) (value T, ok bool) {
	select {
	case <-w.Ready():
		w.mu.Lock()
		val := w.val
		w.mu.Unlock()

		return val, true
	case <-ctx.Done():
		var zero T
		return zero, false
	}
}

func (w *ValueWatcher[T]) newValue(value T) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.val = value

	select {
	case <-w.ch:
		// Channel has already been closed.
	default:
		close(w.ch)
	}
}
