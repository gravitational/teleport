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

var (
	// ErrFnCacheClosed is returned from Get when the FnCache context is closed
	ErrFnCacheClosed = errors.New("fncache permanently closed")
)

// FnCache is a helper for temporarily storing the results of regularly called functions. This helper is
// used to limit the amount of backend reads that occur while the primary cache is unhealthy.  Most resources
// do not require this treatment, but certain resources (cas, nodes, etc) can be loaded on a per-request
// basis and can cause significant numbers of backend reads if the cache is unhealthy or taking a while to init.
type FnCache struct {
	cfg         FnCacheConfig
	mu          sync.Mutex
	nextCleanup time.Time
	entries     map[interface{}]*fnCacheEntry
}

// cleanupMultiplier is an arbitrary multiplier used to derive the schedule for
// periodic lazy cleanup of expired entries.  This cache is meant to be used to
// store a small number of regularly read keys, so most old values aught to be
// removed upon subsequent reads of the same key.
const cleanupMultiplier time.Duration = 16

type FnCacheConfig struct {
	// TTL is the time to live for cache entries.
	TTL time.Duration
	// Clock is the clock used to determine the current time.
	Clock clockwork.Clock
	// Context is the context used to cancel the cache. All loadfns
	// will be provided this context.
	Context context.Context
}

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

	return nil
}

func NewFnCache(cfg FnCacheConfig) (*FnCache, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &FnCache{
		cfg:     cfg,
		entries: make(map[interface{}]*fnCacheEntry),
	}, nil
}

type fnCacheEntry struct {
	v      interface{}
	e      error
	t      time.Time
	loaded chan struct{}
}

func (c *FnCache) removeExpiredLocked(now time.Time) {
	for key, entry := range c.entries {
		select {
		case <-entry.loaded:
			if now.After(entry.t.Add(c.cfg.TTL)) {
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
func (c *FnCache) Get(ctx context.Context, key interface{}, loadfn func(ctx context.Context) (interface{}, error)) (interface{}, error) {
	select {
	case <-c.cfg.Context.Done():
		return nil, ErrFnCacheClosed
	default:
	}

	c.mu.Lock()

	now := c.cfg.Clock.Now()

	// check if we need to perform periodic cleanup
	if now.After(c.nextCleanup) {
		c.removeExpiredLocked(now)
		c.nextCleanup = now.Add(c.cfg.TTL * cleanupMultiplier)
	}

	entry := c.entries[key]

	needsReload := true

	if entry != nil {
		select {
		case <-entry.loaded:
			needsReload = now.After(entry.t.Add(c.cfg.TTL))
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
			// link the config context with the span from ctx, if one exists,
			// so that the loadfn can be traced appropriately.
			loadCtx := oteltrace.ContextWithSpan(c.cfg.Context, oteltrace.SpanFromContext(ctx))
			entry.v, entry.e = loadfn(loadCtx)
			entry.t = c.cfg.Clock.Now()
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
	case <-c.cfg.Context.Done():
		return nil, ErrFnCacheClosed
	}
}
