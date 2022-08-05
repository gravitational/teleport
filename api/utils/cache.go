/*
Copyright 2022 Gravitational, Inc.

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

import (
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
)

// CacheConfig is the config for Cache.
type CacheConfig struct {
	// Clock is used to control time.
	Clock clockwork.Clock
	// DefaultTTL is the item's default time-to-live duration.
	DefaultTTL time.Duration
	// CleanupInterval is the minimum interval to perform a cleanup.
	CleanupInterval time.Duration
}

// SetDefaults sets default values.
func (c *CacheConfig) SetDefaults() {
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.CleanupInterval == 0 {
		// 10 is an is an arbitrary multiplier used to derive the schedule for
		// periodic lazy cleanup of expired items.
		c.CleanupInterval = c.DefaultTTL * 10
	}
}

// cacheItem is a cache item.
type cacheItem[V any] struct {
	Value     V
	ExpiresAt time.Time
}

// isExpired returns true if the item is expired.
func (item cacheItem[V]) isExpired(now time.Time) bool {
	return !item.ExpiresAt.IsZero() && item.ExpiresAt.Before(now)
}

// Cache is a cache map.
type Cache[K comparable, V any] struct {
	nextCleanup time.Time
	cfg         CacheConfig
	items       map[K]cacheItem[V]
	mu          sync.Mutex
}

// NewCache creates a new cache map.
func NewCache[K comparable, V any](config CacheConfig) *Cache[K, V] {
	config.SetDefaults()

	return &Cache[K, V]{
		cfg:   config,
		items: make(map[K]cacheItem[V]),
	}
}

// Get retrieves item with provided key.
func (c *Cache[K, V]) Get(key K) (value V, found bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.cfg.Clock.Now()
	c.maybeCleanup(now)

	item, found := c.items[key]
	if !found {
		return value, false
	}

	if item.isExpired(now) {
		delete(c.items, key)
		return value, false
	}

	return item.Value, true
}

// Set updates cache with the provided key, the provided value, and the default TTL.
func (c *Cache[K, V]) Set(key K, value V) {
	c.SetWithTTL(key, value, c.cfg.DefaultTTL)
}

// SetWithTTL updates cache with the provided key, value, and TTL. If TTL is
// not positive, item will never expire.
func (c *Cache[K, V]) SetWithTTL(key K, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.cfg.Clock.Now()
	c.maybeCleanup(now)

	item := cacheItem[V]{
		Value: value,
	}

	if ttl > 0 {
		item.ExpiresAt = now.Add(ttl)
	}

	c.items[key] = item
}

// Delete removes the key from the cache.
func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)

	c.maybeCleanup(c.cfg.Clock.Now())
}

// Len return size of the cache.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.maybeCleanup(c.cfg.Clock.Now())
	return len(c.items)
}

// maybeCleanup performs a cleanup when CleanupInterval has passed. This
// function assumes lock is already held outside.
func (c *Cache[K, V]) maybeCleanup(now time.Time) {
	if c.cfg.CleanupInterval <= 0 || now.Before(c.nextCleanup) {
		return
	}

	for key, item := range c.items {
		if item.isExpired(now) {
			delete(c.items, key)
		}
	}

	c.nextCleanup = now.Add(c.cfg.CleanupInterval)
}
