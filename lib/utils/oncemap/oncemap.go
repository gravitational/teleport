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

package oncemap

import (
	"context"
	"sync"
)

// OnceMap provides a concurrency-safe map where each key is lazily initialized once on first read.
type OnceMap[K comparable, V any] struct {
	mu      sync.Mutex
	entries map[K]*entry[V]
	fn      func(K) (V, error)
}

// New builds a new OnceMap from the provided function.
func New[K comparable, V any](fn func(K) (V, error)) *OnceMap[K, V] {
	return &OnceMap[K, V]{
		entries: make(map[K]*entry[V]),
		fn:      fn,
	}
}

type entry[V any] struct {
	value V
	err   error
	ready chan struct{}
}

// Get retrieves the value for the given key, initializing it if it hasn't been set up yet.
func (m *OnceMap[K, V]) Get(ctx context.Context, key K) (V, error) {
	e := m.ensure(key)

	select {
	case <-e.ready:
		return e.value, e.err
	case <-ctx.Done():
		return *new(V), ctx.Err()
	}
}

func (m *OnceMap[K, V]) ensure(key K) *entry[V] {
	m.mu.Lock()
	defer m.mu.Unlock()

	e, ok := m.entries[key]
	if ok {
		return e
	}

	e = &entry[V]{ready: make(chan struct{})}
	m.entries[key] = e

	go func() {
		defer close(e.ready)
		e.value, e.err = m.fn(key)
	}()

	return e
}
