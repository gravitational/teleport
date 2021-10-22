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

package cache

import (
	"context"
	"sync"
	"time"
)

// fnCache is a helper for temporarily storing the results of regularly called functions. This helper is
// used to limit the amount of backend reads that occur while the primary cache is unhealthy.  Most resources
// do not require this treatment, but certain resources (cas, nodes, etc) can be loaded on a per-request
// basis and can cause significant numbers of backend reads if the cache is unhealthy or taking a while to init.
type fnCache struct {
	ttl         time.Duration
	mu          sync.Mutex
	nextCleanup time.Time
	entries     map[interface{}]*fnCacheEntry
}

// cleanupMultiplier is an arbitrary multiplier used to derive the schedule for
// periodic lazy cleanup of expired entries.  This cache is meant to be used to
// store a small number of regularly read keys, so most old values aught to be
// removed upon subsequent reads of the same key.
const cleanupMultiplier time.Duration = 16

func newFnCache(ttl time.Duration) *fnCache {
	return &fnCache{
		ttl:     ttl,
		entries: make(map[interface{}]*fnCacheEntry),
	}
}

type fnCacheEntry struct {
	v      interface{}
	e      error
	t      time.Time
	loaded chan struct{}
}

func (c *fnCache) removeExpiredLocked(now time.Time) {
	for key, entry := range c.entries {
		select {
		case <-entry.loaded:
			if now.After(entry.t.Add(c.ttl)) {
				delete(c.entries, key)
			}
		default:
			// entry is still being loaded
		}
	}
}

// Get loads the result associated with the supplied key.  If no result is currently stored, or the stored result
// was acquired >ttl ago, then loadfn is used to reload it.  Subsequent calls while the value is being loaded/reloaded
// block until the first call updates the entry.  Note that the supplied context can cancel the call to Get, but will
// not cancel loading.  The supplied loadfn should not be canceled just because the specific request happens to have
// been canceled.
func (c *fnCache) Get(ctx context.Context, key interface{}, loadfn func() (interface{}, error)) (interface{}, error) {
	c.mu.Lock()

	now := time.Now()

	// check if we need to perform periodic cleanup
	if now.After(c.nextCleanup) {
		c.removeExpiredLocked(now)
		c.nextCleanup = now.Add(c.ttl * cleanupMultiplier)
	}

	entry := c.entries[key]

	needsReload := true

	if entry != nil {
		select {
		case <-entry.loaded:
			needsReload = now.After(entry.t.Add(c.ttl))
		default:
			// reload is already in progress
			needsReload = false
		}
	}

	if needsReload {
		// insert a new entry with a new loaded channel.  this channel will
		// block subsequent reads, and serve as a memory barrier for the results.
		entry = &fnCacheEntry{
			loaded: make(chan struct{}),
		}
		c.entries[key] = entry
		go func() {
			entry.v, entry.e = loadfn()
			entry.t = time.Now()
			close(entry.loaded)
		}()
	}

	c.mu.Unlock()

	// wait for result to be loaded (this is also a memory barrier)
	select {
	case <-entry.loaded:
		return entry.v, entry.e
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
