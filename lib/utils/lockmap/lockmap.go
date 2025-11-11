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
	lm *LockMap[T]

	key      T
	mx       sync.Mutex
	refCount int
}

// A LockMap is a thread-safe map of ref counted [sync.Mutex] used to synchronize access
// to a set of keys. Entries are created lazily and ref counted so that the lock map
// does not grow unbounded.
type LockMap[T comparable] struct {
	locks map[T]*lock[T]
	mx    sync.Mutex
}

// New returns an empty, initialized [LockMap].
func New[T comparable]() *LockMap[T] {
	return &LockMap[T]{
		locks: map[T]*lock[T]{},
		mx:    sync.Mutex{},
	}
}

// An Unlocker is returned by [LockMap.Lock] and must be called to release a given lock and
// decrement its ref count.
type Unlocker interface {
	Unlock()
}

// Lock acquires a lock for the given key, creating one if needed, and returns an [Unlocker].
// Each lock is ref counted and removed when that count reaches 0. This allows reusing a lock
// when it's under contention while preventing unbounded growth of the lock map.
func (lm *LockMap[T]) Lock(key T) Unlocker {
	// alias the LockMap's mutex to avoid confusion
	mapMx := &lm.mx
	mapMx.Lock()

	l, ok := lm.locks[key]
	if !ok {
		l = &lock[T]{key: key, lm: lm}
		lm.locks[key] = l
	}
	l.refCount++
	mapMx.Unlock()

	l.mx.Lock()
	return l
}

// Unlock releases a lock for the given key. If the ref count for the lock is zero, it is removed
// from the map.
func (l *lock[T]) Unlock() {
	// alias the LockMap's mutex to avoid confusion
	mapMx := &l.lm.mx
	mapMx.Lock()
	l.refCount--
	if l.refCount < 1 {
		delete(l.lm.locks, l.key)
	}
	mapMx.Unlock()
	l.mx.Unlock()
}

// Len returns the number of entries remaining in the [LockMap].
func (lm *LockMap[T]) Len() int {
	lm.mx.Lock()
	defer lm.mx.Unlock()
	return len(lm.locks)
}
