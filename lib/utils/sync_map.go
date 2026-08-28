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

package utils

import (
	"maps"
	"sync"
)

// SyncMap is a generics version of a sync.Map.
type SyncMap[K comparable, V any] struct {
	values map[K]V
	mu     sync.RWMutex
}

// Load returns the value stored in the map for a key.
func (s *SyncMap[K, V]) Load(key K) (value V, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.values == nil {
		return value, false
	}
	value, ok = s.values[key]
	return value, ok
}

// Store sets the value for a key.
func (s *SyncMap[K, V]) Store(key K, value V) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.values == nil {
		s.values = make(map[K]V)
	}
	s.values[key] = value
}

// Delete deletes the value for a key.
func (s *SyncMap[K, V]) Delete(key K) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.values == nil {
		return
	}
	delete(s.values, key)
}

// LoadAndDelete loads the value for a key and deletes it if it exists.
func (s *SyncMap[K, V]) LoadAndDelete(key K) (value V, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.values == nil {
		return value, false
	}
	value, ok = s.values[key]
	if ok {
		delete(s.values, key)
	}
	return value, ok
}

// Range calls a function sequentially for each key and value in the map.
// Caution: The map is ony locked while creating a copy of the map values to
// iterate over; it is *not* locked during the actual iteration over that copy,
// nor while f is being evaluated.
func (s *SyncMap[K, V]) Range(f func(key K, value V) bool) {
	items := s.Clone()

	for key, value := range items {
		if !f(key, value) {
			return
		}
	}
}

// Clear clears the underlying map
func (s *SyncMap[K, V]) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.values != nil {
		clear(s.values)
	}
}

// Set sets the underlying managed map to the supplied value. Note that directly reading
// from or writing to `m` outside of the SyncMap after calling `Set()` may result in a
// data race .
func (s *SyncMap[K, V]) Set(m map[K]V) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.values = m
}

// Clone creates an un-synchronized shallow clone of the protected map
func (s *SyncMap[K, V]) Clone() map[K]V {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return maps.Clone(s.values)
}

// Len fetches the number of items in the map
func (s *SyncMap[K, V]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.values)
}

// Read acquires the map read lock and applies the supplied function the
// underlying map, automatically releasing the lock when done. Prefer `Load()`
// when you want to read a single map value. Only prefer `Read()` when
//   - you need a coherent view of the map across multiple reads, or
//   - ensuring that reference-type map values (e.g. maps, struct on the heap)
//     are not modified while being read
func (s *SyncMap[K, V]) Read(fn func(map[K]V)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	fn(s.values)
}

// Write acquires the map lock and applies the supplied function the
// underlying map, automatically releasing the lock when done.
func (s *SyncMap[K, V]) Write(fn func(map[K]V)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.values == nil {
		s.values = make(map[K]V)
	}
	fn(s.values)
}
