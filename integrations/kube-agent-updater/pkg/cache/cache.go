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
