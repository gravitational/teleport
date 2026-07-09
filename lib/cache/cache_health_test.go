/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

func newCacheHealthTestTracker() (*cacheHealthTracker, *prometheus.GaugeVec) {
	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "test_cache_health"},
		[]string{"target"},
	)
	return newCacheHealthTracker(gauge), gauge
}

func newCacheHealthTestCache(tracker *cacheHealthTracker, target string) *Cache {
	return &Cache{
		Config:                Config{target: target},
		initC:                 make(chan struct{}),
		firstTimeInitC:        make(chan struct{}),
		cancel:                func() {},
		eventsFanout:          services.NewFanoutV2(services.FanoutV2Config{}),
		lowVolumeEventsFanout: utils.NewRoundRobin([]*services.FanoutV2{}),
		healthReporter:        tracker.newReporter(target),
	}
}

// TestCacheHealthMultipleCaches reproduces MetricBug from System.p: two cache
// instances report to one metric, and the last report must not clobber the
// other cache's state.
func TestCacheHealthMultipleCaches(t *testing.T) {
	tracker, gauge := newCacheHealthTestTracker()
	a := newCacheHealthTestCache(tracker, "test")
	b := newCacheHealthTestCache(tracker, "test")

	a.setInitError(nil)
	b.setInitError(assert.AnError)
	require.Equal(t, 1.0, testutil.ToFloat64(gauge.WithLabelValues("test")))

	a.setInitError(assert.AnError)
	require.Equal(t, 0.0, testutil.ToFloat64(gauge.WithLabelValues("test")))

	b.setInitError(nil)
	require.Equal(t, 1.0, testutil.ToFloat64(gauge.WithLabelValues("test")))
}

func TestCacheHealthDeregisterOnClose(t *testing.T) {
	tracker, gauge := newCacheHealthTestTracker()
	a := newCacheHealthTestCache(tracker, "test")
	b := newCacheHealthTestCache(tracker, "test")

	a.setInitError(nil)
	b.setInitError(assert.AnError)
	require.Equal(t, 1.0, testutil.ToFloat64(gauge.WithLabelValues("test")))

	require.NoError(t, a.Close())
	require.Equal(t, 0.0, testutil.ToFloat64(gauge.WithLabelValues("test")))

	// A stopped cache ignores subsequent health changes.
	a.setInitError(nil)
	require.Equal(t, 0.0, testutil.ToFloat64(gauge.WithLabelValues("test")))

	require.NoError(t, b.Close())
	require.Equal(t, 1.0, testutil.ToFloat64(gauge.WithLabelValues("test")))
}

func TestCacheHealthReportRaceWithClose(t *testing.T) {
	for range 100 {
		tracker, gauge := newCacheHealthTestTracker()
		reporter := tracker.newReporter("test")
		reporter.report(true)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			reporter.report(false)
		}()
		go func() {
			defer wg.Done()
			reporter.close()
		}()
		wg.Wait()

		// Either the unhealthy report precedes close and is removed, or it
		// follows close and is ignored. Both schedules leave no live caches.
		require.Equal(t, 1.0, testutil.ToFloat64(gauge.WithLabelValues("test")))
	}
}

type cacheHealthAction struct {
	cache   int
	stop    bool
	healthy bool
}

var cacheHealthActions = []cacheHealthAction{
	{cache: 0, healthy: false},
	{cache: 0, healthy: true},
	{cache: 0, stop: true},
	{cache: 1, healthy: false},
	{cache: 1, healthy: true},
	{cache: 1, stop: true},
}

// TestCacheHealthProperty mirrors Driver.p with two caches receiving random
// health changes and stops. After every action it checks Spec.p's expected()
// condition: no live caches or at least one healthy live cache.
func TestCacheHealthProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		tracker, gauge := newCacheHealthTestTracker()
		reporters := [2]*cacheHealthReporter{
			tracker.newReporter("test"),
			tracker.newReporter("test"),
		}
		up := [2]bool{true, true}
		healthy := [2]bool{true, true}
		for _, reporter := range reporters {
			reporter.report(true)
		}

		check := func() {
			want := 0.0
			if (!up[0] && !up[1]) || (up[0] && healthy[0]) || (up[1] && healthy[1]) {
				want = 1.0
			}
			require.Equal(t, want, testutil.ToFloat64(gauge.WithLabelValues("test")))
		}

		actions := rapid.SliceOfN(rapid.SampledFrom(cacheHealthActions), 0, 64).Draw(t, "actions")
		for _, action := range actions {
			if action.stop {
				reporters[action.cache].close()
				up[action.cache] = false
			} else {
				reporters[action.cache].report(action.healthy)
				if up[action.cache] {
					healthy[action.cache] = action.healthy
				}
			}
			check()
		}

		for i, reporter := range reporters {
			reporter.close()
			up[i] = false
			check()
		}
	})
}
