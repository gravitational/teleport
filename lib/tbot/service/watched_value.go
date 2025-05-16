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
		ch := item.Value.(chan struct{})

		select {
		case ch <- struct{}{}:
		default:
			// Notification channels have a buffer size of one, so even if there's
			// no receiver we won't drop the notification.
		}
	}

	return true
}

// ChangeNotifications returns a channel that can be used to watch the value for
// changes. Callers must call the returned close function when they're done.
func (v *WatchedValue[T]) ChangeNotifications() (<-chan struct{}, func()) {
	v.mu.Lock()
	defer v.mu.Unlock()

	ch := make(chan struct{}, 1)
	elem := v.watchers.PushBack(ch)

	close := func() {
		v.mu.Lock()
		defer v.mu.Unlock()
		v.watchers.Remove(elem)
	}

	return ch, close
}
