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
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	apiutils "github.com/gravitational/teleport/api/utils"
)

func TestFnCache_New(t *testing.T) {
	t.Parallel()

	cases := []struct {
		desc      string
		config    FnCacheConfig
		assertion require.ErrorAssertionFunc
	}{
		{
			desc:      "invalid ttl",
			config:    FnCacheConfig{TTL: 0},
			assertion: require.Error,
		},

		{
			desc:      "valid ttl",
			config:    FnCacheConfig{TTL: time.Second},
			assertion: require.NoError,
		},
	}

	for _, tt := range cases {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := NewFnCache(tt.config)
			tt.assertion(t, err)
		})
	}
}

type result struct {
	val any
	err error
}

func TestFnCacheGet(t *testing.T) {
	cache, err := NewFnCache(FnCacheConfig{
		TTL:     time.Second,
		Clock:   clockwork.NewFakeClock(),
		Context: context.Background(),
	})
	require.NoError(t, err)

	value, err := FnCacheGet(context.Background(), cache, "test", func(ctx context.Context) (any, error) {
		return 123, nil
	})
	require.NoError(t, err)
	require.Equal(t, 123, value)

	value2, err := FnCacheGet(context.Background(), cache, "test", func(ctx context.Context) (int, error) {
		return value.(int), nil
	})
	require.NoError(t, err)
	require.Equal(t, 123, value2)

	value3, err := FnCacheGet(context.Background(), cache, "test", func(ctx context.Context) (string, error) {
		return "123", nil
	})
	require.ErrorIs(t, err, trace.BadParameter("value retrieved was int, expected string"))
	require.Empty(t, value3)
}

// TestFnCacheConcurrentReads verifies that many concurrent reads result in exactly one
// value being actually loaded via loadfn if a reasonably long TTL is used.
func TestFnCacheConcurrentReads(t *testing.T) {
	const workers = 100
	t.Parallel()

	ctx := t.Context()

	// set up a chage that won't ttl out values during the test
	cache, err := NewFnCache(FnCacheConfig{TTL: time.Hour})
	require.NoError(t, err)

	results := make(chan result, workers)

	for i := range workers {
		go func(n int) {
			val, err := FnCacheGet(ctx, cache, "key", func(context.Context) (any, error) {
				// return a unique value for each worker so that we can verify whether
				// the values we get come from the same loadfn or not.
				return fmt.Sprintf("val-%d", n), nil
			})
			results <- result{val, err}
		}(i)
	}

	first := <-results
	require.NoError(t, first.err)

	val := first.val.(string)
	require.NotEmpty(t, val)

	for range workers - 1 {
		r := <-results
		require.NoError(t, r.err)
		require.Equal(t, val, r.val.(string))
	}
}

// TestFnCacheExpiry verfies basic expiry.
func TestFnCacheExpiry(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	clock := clockwork.NewFakeClock()

	cache, err := NewFnCache(FnCacheConfig{TTL: time.Hour, Clock: clock, CleanupInterval: time.Second})
	require.NoError(t, err)

	// get is helper for checking if we hit/miss
	get := func() (load bool) {
		val, err := FnCacheGet(ctx, cache, "key", func(context.Context) (string, error) {
			load = true
			return "val", nil
		})
		require.NoError(t, err)
		require.Equal(t, "val", val)
		return
	}

	ttlGet := func() (load bool) {
		val, err := FnCacheGetWithTTL(ctx, cache, "key2", 20*time.Minute, func(context.Context) (string, error) {
			load = true
			return "val2", nil
		})
		require.NoError(t, err)
		require.Equal(t, "val2", val)
		return
	}

	// first get runs the loadfn
	require.True(t, get())

	// subsequent gets use the cached value
	for range 20 {
		require.False(t, get())
	}

	clock.Advance(61 * time.Minute)

	// value has ttl'd out, loadfn is run again
	require.True(t, get())
	require.True(t, ttlGet())

	clock.Advance(time.Minute)

	// and now we're back to hitting a cached value
	require.False(t, get())
	require.False(t, ttlGet())

	clock.Advance(21 * time.Minute)

	// the item with the custom ttl should be loaded while the
	// other item should still be cached
	require.False(t, get())
	require.True(t, ttlGet())

	clock.Advance(10 * time.Minute)

	// we're still hitting a cached value
	require.False(t, get())
	require.False(t, ttlGet())
}

// TestFnCacheFuzzy runs basic FnCache test cases that rely on fuzzy logic and timing to detect
// success/failure. This test isn't really suitable for running in our CI env due to its sensitivery
// to fluxuations in perf, but is arguably a *better* test in that it more accurately simulates real
// usage. This test should be run locally with TEST_FNCACHE_FUZZY=yes when making changes.
func TestFnCacheFuzzy(t *testing.T) {
	if run, _ := apiutils.ParseBool(os.Getenv("TEST_FNCACHE_FUZZY")); !run {
		t.Skip("Test disabled in CI. Enable it by setting env variable TEST_FNCACHE_FUZZY=yes")
	}

	tts := []struct {
		ttl   time.Duration
		delay time.Duration
		desc  string
	}{
		{ttl: time.Millisecond * 40, delay: time.Millisecond * 20, desc: "long ttl, short delay"},
		{ttl: time.Millisecond * 20, delay: time.Millisecond * 40, desc: "short ttl, long delay"},
		{ttl: time.Millisecond * 40, delay: time.Millisecond * 40, desc: "long ttl, long delay"},
		{ttl: time.Millisecond * 40, delay: 0, desc: "non-blocking"},
	}

	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			testFnCacheFuzzy(t, tt.ttl, tt.delay)
		})
	}
}

// testFnCacheFuzzy runs a basic test case which spams concurrent request against a cache
// and verifies that the resulting hit/miss numbers roughly match our expectation.
func testFnCacheFuzzy(t *testing.T, ttl time.Duration, delay time.Duration) {
	const rate = int64(20)     // get attempts per worker per ttl period
	const workers = int64(100) // number of concurrent workers
	const rounds = int64(10)   // number of full ttl cycles to go through

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cache, err := NewFnCache(FnCacheConfig{TTL: ttl})
	require.NoError(t, err)

	// readCounter is incremented upon each cache miss.
	var readCounter atomic.Int64

	// getCounter is incremented upon each get made against the cache, hit or miss.
	var getCounter atomic.Int64

	readTime := make(chan time.Time, 1)

	var wg sync.WaitGroup

	// spawn workers
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(ttl / time.Duration(rate))
			defer ticker.Stop()
			done := time.After(ttl * time.Duration(rounds))
			lastValue := int64(0)
			for {
				select {
				case <-ticker.C:
				case <-done:
					return
				}
				vi, err := FnCacheGet(ctx, cache, "key", func(context.Context) (int64, error) {
					if delay > 0 {
						<-time.After(delay)
					}

					select {
					case readTime <- time.Now():
					default:
					}

					val := readCounter.Add(1)
					return val, nil
				})
				require.NoError(t, err)
				require.GreaterOrEqual(t, vi, lastValue)
				lastValue = vi
				getCounter.Add(1)
			}
		}()
	}

	startTime := <-readTime

	// wait for workers to finish
	wg.Wait()

	elapsed := time.Since(startTime)

	// approxReads is the approximate expected number of reads
	approxReads := float64(elapsed) / float64(ttl+delay)

	// verify that number of actual reads is within +/- 2 of the number of expected reads.
	require.InDelta(t, approxReads, readCounter.Load(), 2)
}

// TestFnCacheCancellation verifies expected cancellation behavior.  Specifically, we expect that
// in-progress loading continues, and the entry is correctly updated, even if the call to Get
// which happened to trigger the load needs to be unblocked early.
func TestFnCacheCancellation(t *testing.T) {
	t.Parallel()

	const longTimeout = time.Second * 10 // should never be hit

	cache, err := NewFnCache(FnCacheConfig{TTL: time.Minute})
	require.NoError(t, err)

	// used to artificially block the load function
	blocker := make(chan struct{})

	// set up a context that we can cancel from within the load function to
	// simulate a scenario where the calling context is canceled or times out.
	// if we actually hit the timeout, that is a bug.
	ctx, cancel := context.WithTimeout(context.Background(), longTimeout)
	defer cancel()

	v, err := FnCacheGet(ctx, cache, "key", func(context.Context) (string, error) {
		cancel()
		<-blocker
		return "val", nil
	})

	require.Empty(t, v)
	require.Equal(t, context.Canceled, trace.Unwrap(err), "context should have been canceled immediately")

	// unblock the loading operation which is still in progress
	close(blocker)

	// since we unblocked the loadfn, we expect the next Get to return almost
	// immediately.  we still use a fairly long timeout just to ensure that failure
	// is due to an actual bug and not due to resource constraints in the test env.
	ctx, cancel = context.WithTimeout(context.Background(), longTimeout)
	defer cancel()

	var loadFnWasRun atomic.Bool
	v, err = FnCacheGet(ctx, cache, "key", func(context.Context) (string, error) {
		loadFnWasRun.Store(true)
		return "", nil
	})

	require.False(t, loadFnWasRun.Load(), "loadfn should not have been run")

	require.NoError(t, err)
	require.Equal(t, "val", v)
}

func TestFnCacheContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cache, err := NewFnCache(FnCacheConfig{
		TTL:     time.Minute,
		Context: ctx,
	})
	require.NoError(t, err)

	_, err = FnCacheGet(context.Background(), cache, "key", func(context.Context) (any, error) {
		return "val", nil
	})
	require.NoError(t, err)

	cancel()

	_, err = FnCacheGet(context.Background(), cache, "key", func(context.Context) (any, error) {
		return "val", nil
	})
	require.ErrorIs(t, err, ErrFnCacheClosed)
}

func TestFnCacheReloadOnErr(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	cache, err := NewFnCache(FnCacheConfig{
		TTL:         time.Minute,
		ReloadOnErr: true,
	})
	require.NoError(t, err)

	var happy, sad atomic.Int64

	// test synchronous case, all sad path loads should result in
	// calls to loadfn.
	for range 100 {
		FnCacheGet(ctx, cache, "happy", func(ctx context.Context) (string, error) {
			happy.Add(1)
			return "yay!", nil
		})

		FnCacheGet(ctx, cache, "sad", func(ctx context.Context) (string, error) {
			sad.Add(1)
			return "", fmt.Errorf("uh-oh")
		})
	}
	require.Equal(t, int64(1), happy.Load())
	require.Equal(t, int64(100), sad.Load())

	// test concurrent case. some "sad" loads should overlap now.
	var wg sync.WaitGroup
	for range 100 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			FnCacheGet(ctx, cache, "happy", func(ctx context.Context) (string, error) {
				happy.Add(1)
				return "yay!", nil
			})
		}()

		go func() {
			defer wg.Done()
			FnCacheGet(ctx, cache, "sad", func(ctx context.Context) (string, error) {
				sad.Add(1)
				return "", fmt.Errorf("uh-oh")
			})
		}()
	}
	require.Equal(t, int64(1), happy.Load())
	require.Greater(t, int64(200), sad.Load())
}

func TestFnCacheEviction(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	clock := clockwork.NewFakeClock()

	type item struct {
		k any
		v any
	}
	expiredC := make(chan item, 5)
	cache, err := NewFnCache(FnCacheConfig{
		TTL:     time.Hour,
		Context: ctx,
		Clock:   clock,
		OnExpiry: func(ctx context.Context, key, expired any) {
			expiredC <- item{k: key, v: expired}
		},
	})
	require.NoError(t, err)

	// Populate the cache with items that have varying TTL.
	out, err := FnCacheGet(ctx, cache, "test", func(ctx context.Context) (int, error) {
		return 100, nil
	})
	require.NoError(t, err)
	require.Equal(t, 100, out)

	out2, err := FnCacheGetWithTTL(ctx, cache, 100, 24*time.Hour, func(ctx context.Context) (string, error) {
		return "test", nil
	})
	require.NoError(t, err)
	require.Equal(t, "test", out2)

	// Assert that eviction does not occur prematurely.
	for range 6 {
		clock.Advance(10 * time.Minute)
		cache.RemoveExpired()

		select {
		case item := <-expiredC:
			t.Fatalf("item %v was prematurely expired from the cache", item.k)
		default:
		}
	}

	// Assert that eviction occurs for the expected resource.
	clock.Advance(10 * time.Minute)
	cache.RemoveExpired()
	select {
	case expired := <-expiredC:
		key, ok := expired.k.(string)
		require.True(t, ok)
		require.Equal(t, "test", key)
		val, ok := expired.v.(int)
		require.True(t, ok)
		require.Equal(t, 100, val)
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for item to be expired from the cache")
	}

	// Assert that eviction never occurs for the other resource.
	select {
	case item := <-expiredC:
		t.Fatalf("item %v was prematurely expired from the cache", item.k)
	default:
	}

	// Add a value with the default TTL again.
	out, err = FnCacheGet(ctx, cache, "test", func(ctx context.Context) (int, error) {
		return 100, nil
	})
	require.NoError(t, err)
	require.Equal(t, 100, out)

	// Shutdown the cache and validate all items are expired.
	cache.Shutdown(context.Background())
	timeout := time.After(10 * time.Second)
	for range 2 {
		select {
		case expired := <-expiredC:
			switch k := expired.k.(type) {
			case string:
				require.Equal(t, "test", k)
				val, ok := expired.v.(int)
				require.True(t, ok)
				require.Equal(t, 100, val)
			case int:
				require.Equal(t, 100, k)
				val, ok := expired.v.(string)
				require.True(t, ok)
				require.Equal(t, "test", val)
			}

		case <-timeout:
			t.Fatalf("timed out waiting for item to be expired from the cache")
		}
	}

	// Assert that once the cache is shutdown that it does not accept any new values.
	_, err = FnCacheGet(ctx, cache, "test", func(ctx context.Context) (int, error) {
		return 100, nil
	})
	require.ErrorIs(t, err, ErrFnCacheClosed)
}

func TestFnCacheRemove(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	type item struct {
		k any
		v any
	}
	expiredC := make(chan item, 5)
	cache, err := NewFnCache(FnCacheConfig{
		TTL:     time.Hour,
		Context: ctx,
		OnExpiry: func(ctx context.Context, key, expired any) {
			expiredC <- item{k: key, v: expired}
		},
	})
	require.NoError(t, err)

	// Populate an entry in the cache.
	out, err := FnCacheGet(ctx, cache, "test", func(ctx context.Context) (int, error) {
		return 100, nil
	})
	require.NoError(t, err)
	require.Equal(t, 100, out)

	// Retrieve the entry and validate the loadFn isn't called
	// and that the previously stored value is returned instead.
	out, err = FnCacheGet(ctx, cache, "test", func(ctx context.Context) (int, error) {
		return 0, nil
	})
	require.NoError(t, err)
	require.Equal(t, 100, out)

	// Remove the entry explicitly.
	cache.Remove("test")

	// Retrieve the entry again, this time the loadFn should
	// be called because the item was explicitly removed.
	out, err = FnCacheGet(ctx, cache, "test", func(ctx context.Context) (int, error) {
		return 0, nil
	})
	require.NoError(t, err)
	require.Equal(t, 0, out)
}

func TestFnCacheSet(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	clock := clockwork.NewFakeClock()
	type item struct {
		k any
		v any
	}
	expiredC := make(chan item, 5)
	cache, err := NewFnCache(FnCacheConfig{
		TTL:     time.Hour,
		Context: ctx,
		Clock:   clock,
		OnExpiry: func(ctx context.Context, key, expired any) {
			expiredC <- item{k: key, v: expired}
		},
	})
	require.NoError(t, err)

	// Populate an entry in the cache.
	out, err := FnCacheGet(ctx, cache, "test", func(ctx context.Context) (int, error) {
		return 100, nil
	})
	require.NoError(t, err)
	require.Equal(t, 100, out)

	// Manually override the value.
	cache.Set("test", 500)

	// Retrieve the item again and validate the loadFn isn't called
	// and our manually set value is returned.
	out, err = FnCacheGet(ctx, cache, "test", func(ctx context.Context) (int, error) {
		return 100, nil
	})
	require.NoError(t, err)
	require.Equal(t, 500, out)

	// Time travel to expire the item from the cache.
	clock.Advance(2 * time.Hour)

	// Retrieve the item again and validate the loadFn is called
	// since the old item should have expired
	out, err = FnCacheGet(ctx, cache, "test", func(ctx context.Context) (int, error) {
		return 100, nil
	})
	require.NoError(t, err)
	require.Equal(t, 100, out)

	// Manually override the value with a TTL this time.
	cache.SetWithTTL("test", 999, time.Minute)

	// Retrieve the item again and validate the loadFn isn't called
	// and our manually set value is returned.
	out, err = FnCacheGet(ctx, cache, "test", func(ctx context.Context) (int, error) {
		return 100, nil
	})
	require.NoError(t, err)
	require.Equal(t, 999, out)

	// Time travel to expire the item from the cache.
	clock.Advance(2 * time.Minute)

	// Retrieve the item again and validate the loadFn is called
	// since the old item should have expired
	out, err = FnCacheGet(ctx, cache, "test", func(ctx context.Context) (int, error) {
		return 100, nil
	})
	require.NoError(t, err)
	require.Equal(t, 100, out)
}
