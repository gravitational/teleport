// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package oraclejoin

import (
	"context"
	"crypto/x509"
	"sync"
	"time"
)

// RootCACache caches Oracle instance root CAs for each region. It is inspired
// by lib/utils.FnCache which is designed for caching backend reads, but this
// cache makes a couple different choices:
//   - Each cache entry expiry is determined by the response we get from the
//     Oracle API and unknown before making the request.
//   - Error results are always retried at the next request.
//   - There is no regularly scheduled cleanup, expired entries will be cleaned
//     up on the next get. This cache will not be large, max 1 entry per OCI region.
type RootCACache struct {
	entries map[string]*rootCACacheEntry
	mu      sync.Mutex
}

// NewRootCACache returns a new RootCACache, ready to use.
func NewRootCACache() *RootCACache {
	return &RootCACache{
		entries: make(map[string]*rootCACacheEntry),
	}
}

type getRootCAPoolFn func() (*x509.CertPool, time.Time, error)

// get is the only method consumers of RootCACache should call. Given a key
// (which should be the region of the root CA) and a getRootCAPoolFn, it will
// return a cached root CA pool if one is present and not expired, else it will
// call loadFn to request the root CA pool.
func (c *RootCACache) get(ctx context.Context, key string, loadFn getRootCAPoolFn) (*x509.CertPool, error) {
	entry := c.getEntry(key, loadFn)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-entry.loaded:
		return entry.caPool, entry.err
	}
}

func (c *RootCACache) getEntry(key string, loadFn getRootCAPoolFn) *rootCACacheEntry {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	c.removeExpiredLocked(now)

	if entry, ok := c.entries[key]; ok {
		// There is an existing cache entry, return it. If the entry is expired
		// or stores an error it would have been removed in removeExpiredLocked
		// above.
		return entry
	}

	// Create a new cache entry to store and return.
	entry := &rootCACacheEntry{
		loaded: make(chan struct{}),
	}
	c.entries[key] = entry
	go func() {
		entry.caPool, entry.expires, entry.err = loadFn()
		close(entry.loaded)
	}()

	return entry
}

func (c *RootCACache) removeExpiredLocked(now time.Time) {
	for key, entry := range c.entries {
		select {
		case <-entry.loaded:
			if entry.expires.Before(now) || entry.err != nil {
				delete(c.entries, key)
			}
		default:
			// entry is still being loaded.
			continue
		}
	}
}

type rootCACacheEntry struct {
	// loaded will be closed when the entry is ready. It also acts as a memory barrier:
	// the below fields must only be written before loaded is closed, and must
	// only be read after it is closed.
	loaded  chan struct{}
	caPool  *x509.CertPool
	expires time.Time
	err     error
}
