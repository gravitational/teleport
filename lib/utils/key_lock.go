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

package utils

import "sync"

// KeyLock provides per-key mutual exclusion. Only one caller per key can
// proceed at a time, while unrelated keys are processed concurrently.
//
// The zero value is ready to use. A KeyLock must not be copied after first use.
type KeyLock[K comparable] struct {
	mu sync.Mutex
	m  map[K]*keyLockEntry
}

type keyLockEntry struct {
	mu       sync.Mutex
	refCount int
}

// Lock locks the given key. It blocks until the key is available.
func (k *KeyLock[K]) Lock(key K) {
	k.mu.Lock()
	if k.m == nil {
		k.m = make(map[K]*keyLockEntry)
	}
	entry, exists := k.m[key]
	if !exists {
		entry = &keyLockEntry{}
		k.m[key] = entry
	}
	entry.refCount++
	k.mu.Unlock()

	entry.mu.Lock()
}

// Unlock unlocks the given key. It is a runtime error if the key is not
// locked on entry to Unlock.
func (k *KeyLock[K]) Unlock(key K) {
	k.mu.Lock()
	defer k.mu.Unlock()

	entry := k.m[key]
	if entry == nil {
		panic("keylock: unlock of unlocked key")
	}
	if entry.refCount == 0 {
		panic("keylock: ref count is zero (this is a bug)")
	}

	entry.mu.Unlock()

	entry.refCount--
	if entry.refCount == 0 {
		delete(k.m, key)
	}
}

// Acquire locks the given key and returns an unlock function. The unlock
// function is safe to call multiple times only the first call has any effect.
func (k *KeyLock[K]) Acquire(key K) func() {
	k.Lock(key)
	return sync.OnceFunc(func() {
		k.Unlock(key)
	})
}
