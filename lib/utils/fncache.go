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
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// NOTE: when making changes to this file, run tests with `TEST_FNCACHE_FUZZY=yes` to enable
// additional fuzzy tests which aren't run during normal CI.

// ErrFnCacheClosed is returned from Get when the FnCache context is closed
var ErrFnCacheClosed = errors.New("fncache permanently closed")

// FnCache is a helper for temporarily storing the results of regularly called functions. This helper is
// used to limit the amount of backend reads that occur while the primary cache is unhealthy.  Most resources
// do not require this treatment, however, certain resources (cas, nodes, etc.) can be loaded on a per-request
// basis and can cause a significant number of backend reads if the cache is unhealthy or taking a while to initialize.
type FnCache struct {
	cfg         FnCacheConfig
	closed      bool
	cancel      context.CancelFunc
	nextCleanup time.Time

	mu      sync.Mutex
	entries map[any]*fnCacheEntry
}

// cleanupMultiplier is an arbitrary multiplier used to derive the default interval
// for periodic lazy cleanup of expired entries. This cache is typically used to
// store a small number of regularly read keys, so most old values aught to be
// removed upon subsequent reads of the same key. If the cache is being used in a
// context where keys might become regularly orphaned (no longer read), then a
// custom CleanupInterval should be provided.
const cleanupMultiplier time.Duration = 16

// FnCacheConfig contains dependencies for a FnCache.
type FnCacheConfig struct {
	// TTL is the time to live for cache entries.
	TTL time.Duration
	// Clock is the clock used to determine the current time.
	Clock clockwork.Clock
	// Context is the context used to cancel the cache. All loadfns
	// will be provided with this context.
	Context context.Context
	// ReloadOnErr causes entries to be reloaded immediately if
	// the currently loaded value is an error. Note that all concurrent
	// requests registered before load completes still observe the
	// same error. This option is only really useful for longer TTLs.
	ReloadOnErr bool
	// CleanupInterval is the interval at which cleanups occur (defaults to
	// 16x the supplied TTL). Longer cleanup intervals are appropriate for
	// caches where keys are unlikely to become orphaned. Shorter cleanup
	// intervals should be used when keys regularly become orphaned.
	CleanupInterval time.Duration
	// OnExpiry is an optional callback that will be executed any time
	// an item is expired and removed from the cache.
	OnExpiry func(ctx context.Context, key any, value any)
}

// CheckAndSetDefaults validates the FnCacheConfig is populated
// with required fields and sets any omitted fields to default values.
func (c *FnCacheConfig) CheckAndSetDefaults() error {
	if c.TTL <= 0 {
		return trace.BadParameter("missing TTL parameter")
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	if c.Context == nil {
		c.Context = context.Background()
	}

	if c.CleanupInterval <= 0 {
		c.CleanupInterval = c.TTL * cleanupMultiplier
	}

	return nil
}

// NewFnCache creates a [FnCache] from the provided [FnCacheConfig].
func NewFnCache(cfg FnCacheConfig) (*FnCache, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cache := &FnCache{
		cfg:     cfg,
		entries: make(map[any]*fnCacheEntry),
	}

	cache.cfg.Context, cache.cancel = context.WithCancel(cfg.Context)

	return cache, nil

}

type fnCacheEntry struct {
	v      any
	e      error
	t      time.Time
	ttl    time.Duration
	loaded chan struct{}
}

// Shutdown expires all items in the cache. If the OnExpires
// callback was set in the FnCacheConfig it will be called once
// per item in the cache.
func (c *FnCache) Shutdown(ctx context.Context) {
	c.mu.Lock()
	c.cancel()
	c.closed = true
	entries := c.entries
	c.entries = make(map[any]*fnCacheEntry)
	c.mu.Unlock()

	// non-blocking eviction
	for key, entry := range entries {
		select {
		case <-entry.loaded:
			if c.cfg.OnExpiry != nil && entry.e == nil {
				c.cfg.OnExpiry(ctx, key, entry.v)
			}

			delete(entries, key)
		case <-ctx.Done():
			return
		default:
			// entry is still being loaded
		}
	}

	// blocking eviction
	for key, entry := range entries {
		select {
		case <-entry.loaded:
			if c.cfg.OnExpiry != nil && entry.e == nil {
				c.cfg.OnExpiry(ctx, key, entry.v)
			}

			delete(entries, key)
		case <-ctx.Done():
			return
		}
	}
}

// Remove purges a specific item in the cache.
func (c *FnCache) Remove(key any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// Set places an item in the cache using the default TTL.
func (c *FnCache) Set(key, value any) {
	c.SetWithTTL(key, value, c.cfg.TTL)
}

// SetWithTTL places an item in the cache with an explicit TTL.
func (c *FnCache) SetWithTTL(key, value any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	loaded := make(chan struct{})
	close(loaded)

	c.entries[key] = &fnCacheEntry{
		v:      value,
		t:      c.cfg.Clock.Now(),
		ttl:    ttl,
		loaded: loaded,
	}
}

// RemoveExpired purges any items from the cache which have exceeded their TTL.
func (c *FnCache) RemoveExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.cfg.Clock.Now()
	c.removeExpiredLocked(now)
	c.nextCleanup = now.Add(c.cfg.CleanupInterval)
}

func (c *FnCache) removeExpiredLocked(now time.Time) {
	for key, entry := range c.entries {
		select {
		case <-entry.loaded:
			if now.After(entry.t.Add(entry.ttl)) {
				if c.cfg.OnExpiry != nil && entry.e == nil {
					c.cfg.OnExpiry(c.cfg.Context, key, entry.v)
				}

				delete(c.entries, key)
			}
		default:
			// entry is still being loaded
		}
	}
}

// FnCacheGet loads the result associated with the supplied key. If no result is currently stored, or the stored result
// was acquired >TTL ago, then loadfn is used to reload it. Subsequent calls while the value is being loaded/reloaded
// block until the first call updates the entry. Note that the supplied context can cancel the call to Get, but will
// not cancel loading. The supplied loadfn should not be canceled just because the specific request happens to have
// been canceled.
func FnCacheGet[T any](ctx context.Context, cache *FnCache, key any, loadfn func(ctx context.Context) (T, error)) (T, error) {
	return FnCacheGetWithTTL(ctx, cache, key, cache.cfg.TTL, loadfn)
}

// FnCacheGetWithTTL is identical to FnCacheGet except that it allows individual keys to specify
// a TTL that is used instead of the configured TTL for the FnCache.
func FnCacheGetWithTTL[T any](ctx context.Context, cache *FnCache, key any, ttl time.Duration, loadfn func(ctx context.Context) (T, error)) (T, error) {
	t, err := cache.get(ctx, key, ttl, func(ctx context.Context) (any, error) {
		return loadfn(ctx)
	})

	ret, ok := t.(T)
	switch {
	case err != nil:
		return ret, err
	case t == nil:
		return ret, nil
	case !ok:
		return ret, trace.BadParameter("value retrieved was %T, expected %T", t, ret)
	}

	return ret, err
}

// get loads the result associated with the supplied key. If no result is currently stored, or the stored result
// was acquired >ttl ago, then loadfn is used to reload it. Subsequent calls while the value is being loaded/reloaded
// block until the first call updates the entry. Note that the supplied context can cancel the call to Get, but will
// not cancel loading. The supplied loadfn should not be canceled just because the specific request happens to have
// been canceled.
func (c *FnCache) get(ctx context.Context, key any, ttl time.Duration, loadfn func(ctx context.Context) (any, error)) (any, error) {
	select {
	case <-c.cfg.Context.Done():
		return nil, ErrFnCacheClosed
	default:
	}

	c.mu.Lock()

	if c.closed {
		c.mu.Unlock()
		return nil, ErrFnCacheClosed
	}

	now := c.cfg.Clock.Now()

	// Check if we need to perform periodic cleanup.
	if now.After(c.nextCleanup) {
		c.removeExpiredLocked(now)
		c.nextCleanup = now.Add(c.cfg.CleanupInterval)
	}

	entry := c.entries[key]

	needsReload := true

	if entry != nil {
		select {
		case <-entry.loaded:
			needsReload = now.After(entry.t.Add(entry.ttl))
			if c.cfg.ReloadOnErr && entry.e != nil {
				needsReload = true
			}
		default:
			// reload is already in progress
			needsReload = false
		}
	}

	if needsReload {
		// Insert a new entry with a new loaded channel. This channel will
		// block subsequent reads, and serve as a memory barrier for the results.
		entry = &fnCacheEntry{
			loaded: make(chan struct{}),
			ttl:    ttl,
		}
		c.entries[key] = entry
		go func() {
			// Link the config context with the span from ctx, if one exists,
			// so that the loadfn can be traced appropriately.
			loadCtx := oteltrace.ContextWithSpan(c.cfg.Context, oteltrace.SpanFromContext(ctx))
			entry.v, entry.e = loadfn(loadCtx)
			entry.t = c.cfg.Clock.Now()
			close(entry.loaded)
		}()
	}

	c.mu.Unlock()

	// Wait for the result to be loaded (this is also a memory barrier).
	select {
	case <-entry.loaded:
		return entry.v, entry.e
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.cfg.Context.Done():
		return nil, ErrFnCacheClosed
	}
}
