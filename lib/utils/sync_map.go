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

// Range calls a function sequentially for each key and value in the map. Note
// that the map is not locked between evaluations of f.
func (s *SyncMap[K, V]) Range(f func(key K, value V) bool) {
	s.mu.RLock()
	items := maps.Clone(s.values)
	s.mu.RUnlock()

	for key, value := range items {
		if !f(key, value) {
			return
		}
	}
}
