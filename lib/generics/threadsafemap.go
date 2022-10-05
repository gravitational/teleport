/*
Copyright 2021 Gravitational, Inc.

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

package generics

import "sync"

// ThreadSafeMap is a generic, mutex protected map
type ThreadSafeMap[K comparable, V any] struct {
	map_ map[K]V
	sync.Mutex
}

func NewThreadSafeMap[K comparable, V any]() ThreadSafeMap[K, V] {
	return ThreadSafeMap[K, V]{
		map_: make(map[K]V),
	}
}

// Set sets key to val
func (m *ThreadSafeMap[K, V]) Set(key K, val V) {
	m.Lock()
	defer m.Unlock()

	m.map_[key] = val
}

// Get gets the value val at key. If a value at key exists,
// ok is set to true, if not it's set to false.
func (m *ThreadSafeMap[K, V]) Get(key K) (val V, ok bool) {
	m.Lock()
	defer m.Unlock()

	val, ok = m.map_[key]
	return
}

// Delete deletes the key value pair at key.
// If there is no such key, Delete is a no-op.
func (m *ThreadSafeMap[K, V]) Delete(key K) {
	m.Lock()
	defer m.Unlock()
	delete(m.map_, key)
}
