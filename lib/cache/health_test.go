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

package cache

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// newHealthGauge returns a gauge shaped like the real cacheHealth metric for
// use in tests.
func newHealthGauge() *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: "cache",
		Name:      "health",
	}, []string{teleport.TagCacheComponent})
}

func healthValue(t *testing.T, gauge *prometheus.GaugeVec, target string) float64 {
	t.Helper()
	return testutil.ToFloat64(gauge.WithLabelValues(target))
}

func healthSeriesCount(t *testing.T, gauge *prometheus.GaugeVec) int {
	t.Helper()
	return testutil.CollectAndCount(gauge)
}

// healthSeriesForTarget counts the series in gauge whose cache_component label
// matches target. Used for the process-wide gauge, which accumulates series
// from other caches exercised in the package.
func healthSeriesForTarget(t *testing.T, gauge *prometheus.GaugeVec, target string) int {
	t.Helper()
	ch := make(chan prometheus.Metric, 128)
	gauge.Collect(ch)
	close(ch)

	count := 0
	for m := range ch {
		var metric dto.Metric
		require.NoError(t, m.Write(&metric))
		for _, label := range metric.GetLabel() {
			if label.GetName() == teleport.TagCacheComponent && label.GetValue() == target {
				count++
			}
		}
	}
	return count
}

// TestHealthReporter exercises the aggregation logic of the health reporter
// that fixes the shared-metric clobbering race between caches that use the same
// target.
func TestHealthReporter(t *testing.T) {
	const target = "auth"

	t.Run("shutting down instance does not clobber a healthy one", func(t *testing.T) {
		gauge := newHealthGauge()
		r := newHealthReporter(gauge)

		oldCache := &Cache{Config: Config{target: target}}
		newCache := &Cache{Config: Config{target: target}}

		// The old cache reports healthy first, mirroring steady state.
		r.setHealth(oldCache, true)
		require.Equal(t, 1.0, healthValue(t, gauge, target))

		// A new TeleportProcess starts and its cache becomes healthy while the
		// old one is still running (e.g. an External Audit Storage reload).
		r.setHealth(newCache, true)
		require.Equal(t, 1.0, healthValue(t, gauge, target))

		// The old cache's watcher loop errors out during shutdown and reports
		// unhealthy. With the naive per-target approach this clobbered the
		// metric to 0; with per-instance tracking the aggregate stays healthy
		// because the new cache is still healthy.
		r.setHealth(oldCache, false)
		require.Equal(t, 1.0, healthValue(t, gauge, target),
			"a shutting-down cache must not clobber the health of a running one")

		// Once the old cache is fully closed it is removed and only the healthy
		// new cache remains.
		r.remove(oldCache)
		require.Equal(t, 1.0, healthValue(t, gauge, target))
	})

	t.Run("aggregate reflects only running instances", func(t *testing.T) {
		gauge := newHealthGauge()
		r := newHealthReporter(gauge)

		a := &Cache{Config: Config{target: target}}
		b := &Cache{Config: Config{target: target}}

		// b is genuinely unhealthy, a is healthy: the service still has a
		// working cache, so report healthy.
		r.setHealth(a, true)
		r.setHealth(b, false)
		require.Equal(t, 1.0, healthValue(t, gauge, target))

		// The healthy instance shuts down. The value is recomputed from the
		// remaining (unhealthy) instance and now reports unhealthy.
		r.remove(a)
		require.Equal(t, 0.0, healthValue(t, gauge, target))
	})

	t.Run("metric is deleted when the last instance stops", func(t *testing.T) {
		gauge := newHealthGauge()
		r := newHealthReporter(gauge)

		c := &Cache{Config: Config{target: target}}

		r.setHealth(c, false)
		require.Equal(t, 1, healthSeriesCount(t, gauge))

		// When the only cache for a target stops, the series is removed so a
		// stale (unhealthy) value can't linger forever.
		r.remove(c)
		require.Equal(t, 0, healthSeriesCount(t, gauge),
			"the metric should be removed once no caches for the target remain")
	})

	t.Run("remove is safe for unknown and repeated instances", func(t *testing.T) {
		gauge := newHealthGauge()
		r := newHealthReporter(gauge)

		c := &Cache{Config: Config{target: target}}

		// Removing an instance that was never registered is a no-op (e.g. an
		// unstarted cache being closed).
		require.NotPanics(t, func() { r.remove(c) })

		r.setHealth(c, true)
		r.remove(c)
		// A second remove (Close may be called more than once) is a no-op.
		require.NotPanics(t, func() { r.remove(c) })
		require.Equal(t, 0, healthSeriesCount(t, gauge))
	})
}

// newHealthTestCache builds a minimal *Cache with just enough state wired up to
// call setInitError and Close without a backend.
func newHealthTestCache(t *testing.T, target string) *Cache {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &Cache{
		Config:                Config{target: target},
		ctx:                   ctx,
		cancel:                cancel,
		initC:                 make(chan struct{}),
		firstTimeInitC:        make(chan struct{}),
		eventsFanout:          services.NewFanoutV2(services.FanoutV2Config{}),
		lowVolumeEventsFanout: utils.NewRoundRobin([]*services.FanoutV2{services.NewFanoutV2(services.FanoutV2Config{})}),
	}
}

// TestCacheHealthMetricReloadRace reproduces the reported race through the real
// setInitError and Close code paths (using the process-wide cacheHealthReporter
// and cacheHealth gauge). It models an External Audit Storage reload where a new
// TeleportProcess (and its auth cache) starts before the old one is shut down.
//
// Before the fix, the old cache's shutdown wrote 0 to the shared per-target
// gauge after the new cache had written 1, leaving the metric stuck reporting
// unhealthy. This test asserts the metric stays healthy while a healthy cache is
// running and is cleared once all caches for the target stop.
func TestCacheHealthMetricReloadRace(t *testing.T) {
	// Use a target no real cache uses so this test doesn't collide with other
	// caches exercised in the package that share the process-wide gauge.
	const target = "test-health-reload-race"

	oldCache := newHealthTestCache(t, target)
	newCache := newHealthTestCache(t, target)

	// Both caches become healthy (the new process starts while the old is still
	// running).
	oldCache.setInitError(nil)
	newCache.setInitError(nil)
	require.Equal(t, 1.0, healthValue(t, cacheHealth, target))

	// The old cache's watcher loop returns an error as it shuts down and reports
	// unhealthy right before it closes.
	oldCache.setInitError(context.Canceled)
	require.Equal(t, 1.0, healthValue(t, cacheHealth, target),
		"the healthy new cache must keep the metric at 1 despite the old cache reporting unhealthy")

	// The old cache finishes shutting down.
	require.NoError(t, oldCache.Close())
	require.Equal(t, 1.0, healthValue(t, cacheHealth, target))

	// Finally the new cache stops too; the metric is removed rather than left
	// with a stale value.
	require.NoError(t, newCache.Close())
	require.Equal(t, 0, healthSeriesForTarget(t, cacheHealth, target),
		"the metric should be cleared once every cache for the target has stopped")
}
