// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package lockmap

import (
	"sync"
)

type lock[T comparable] struct {
	mu       sync.Mutex
	refCount int
}

// A LockMap is a thread-safe map of ref counted [sync.Mutex] used to synchronize access
// to a set of keys. Entries are created lazily and ref counted so that the lock map
// does not grow unbounded.
type LockMap[T comparable] struct {
	locks map[T]*lock[T]
	mu    sync.Mutex
}

// Lock acquires a lock for the given key, creating one if needed, and returns an [Unlocker].
// Each lock is ref counted and removed when that count reaches 0. This allows reusing a lock
// when it's under contention while preventing unbounded growth of the lock map.
func (lm *LockMap[T]) Lock(key T) {
	// alias the LockMap's mutex to avoid confusion
	mapMu := &lm.mu
	mapMu.Lock()
	if lm.locks == nil {
		lm.locks = make(map[T]*lock[T])
	}
	l, ok := lm.locks[key]
	if !ok {
		l = &lock[T]{}
		lm.locks[key] = l
	}
	l.refCount++
	mapMu.Unlock()
	l.mu.Lock()
}

// Unlock releases a lock for the given key. If the ref count for the lock is zero, it is removed
// from the map.
func (lm *LockMap[T]) Unlock(key T) {
	mapMu := &lm.mu
	mapMu.Lock()
	l, ok := lm.locks[key]
	if !ok {
		return
	}
	l.refCount--
	if l.refCount < 1 {
		delete(lm.locks, key)
	}
	mapMu.Unlock()
	l.mu.Unlock()
}

// Len returns the number of entries remaining in the [LockMap].
func (lm *LockMap[T]) Len() int {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	return len(lm.locks)
}
