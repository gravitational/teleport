/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import "sync"

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
