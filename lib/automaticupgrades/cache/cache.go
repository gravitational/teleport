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

package cache

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// TimedMemoize wraps a function returning (value, error) and caches the
// value AND the error for a specific time. This cache is mainly used to ensure
// external calls rates are reasonable. The cache is thread-safe.
type TimedMemoize[T any] struct {
	clock         clockwork.Clock
	mutex         sync.Mutex
	cachedValue   T
	cachedError   error
	validUntil    time.Time
	cacheDuration time.Duration
	getter        func(ctx context.Context) (T, error)
}

// Get does a cache lookup and updates the cache in case of cache miss.
func (c *TimedMemoize[T]) Get(ctx context.Context) (T, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.validUntil.After(c.clock.Now()) {
		// TimedMemoize hit, we return cached result
		return c.cachedValue, trace.Wrap(c.cachedError)
	}

	// Cache miss, we do a query and update the cache
	value, err := c.getter(ctx)
	c.validUntil = c.clock.Now().Add(c.cacheDuration)
	c.cachedValue = value
	c.cachedError = newCachedError(err, c.validUntil)
	return value, trace.Wrap(err)
}

// NewTimedMemoize builds and returns a TimedMemoize
func NewTimedMemoize[T any](getter func(ctx context.Context) (T, error), duration time.Duration) *TimedMemoize[T] {
	return &TimedMemoize[T]{
		clock:         clockwork.NewRealClock(),
		getter:        getter,
		cacheDuration: duration,
	}
}
