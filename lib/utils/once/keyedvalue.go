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

package once

import (
	"context"
	"sync"
)

type entry[V any] struct {
	value V
	err   error
	ready chan struct{}
}

// KeyedValue provides a keyed, fallible, and context-aware equivalent to [sync.OnceValue]. the
// wrapped function is called exactly once for a given key and the result is cached indefinitely.
// canceling the context passed to the returned closure unblocks the waiting caller but does not cause
// cancellation of inflight calls to the wrapped function. the returned cancel function may optionally
// be called to signal cancellation to inflight and future calls of the wrapped function.
func KeyedValue[K comparable, V any](fn func(context.Context, K) (V, error)) (func(context.Context, K) (V, error), context.CancelFunc) {

	entries := make(map[K]*entry[V])
	var mu sync.Mutex

	closeContext, cancel := context.WithCancel(context.Background())

	ensure := func(key K) *entry[V] {
		mu.Lock()
		defer mu.Unlock()

		e, ok := entries[key]
		if ok {
			return e
		}

		e = &entry[V]{ready: make(chan struct{})}
		entries[key] = e

		go func() {
			defer close(e.ready)
			e.value, e.err = fn(closeContext, key)
		}()
		return e
	}

	return func(ctx context.Context, key K) (V, error) {
		e := ensure(key)

		select {
		case <-e.ready:
			return e.value, e.err
		case <-ctx.Done():
			return *new(V), ctx.Err()
		}
	}, cancel
}
