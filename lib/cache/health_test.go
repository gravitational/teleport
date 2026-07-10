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
	"sync"
	"testing"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
)

func newTestHealthGauge() *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: "cache",
			Name:      "health",
		},
		[]string{teleport.TagCacheComponent},
	)
}

// readGauge reads the current value of the gauge series for target without
// mutating the vector (unlike WithLabelValues, which would create the series).
// The second return value reports whether a series for target exists at all.
func readGauge(t require.TestingT, g *prometheus.GaugeVec, target string) (float64, bool) {
	ch := make(chan prometheus.Metric, 64)
	g.Collect(ch)
	close(ch)
	for m := range ch {
		var parsed dto.Metric
		require.NoError(t, m.Write(&parsed))
		for _, label := range parsed.GetLabel() {
			if label.GetName() == teleport.TagCacheComponent && label.GetValue() == target {
				return parsed.GetGauge().GetValue(), true
			}
		}
	}
	return 0, false
}

// TestReproduce_HealthMetricClobber reproduces (and, with the fix in place,
// guards against) the race where an old cache instance shutting down clobbers
// the shared health metric that a new, still-running cache instance with the
// same target has marked healthy.
func TestReproduce_HealthMetricClobber(t *testing.T) {
	const target = "reproduce-clobber"

	newCache := &Cache{
		id:             nextCacheID(),
		Config:         Config{target: target},
		initC:          make(chan struct{}),
		firstTimeInitC: make(chan struct{}),
	}
	oldCache := &Cache{
		id:             nextCacheID(),
		Config:         Config{target: target},
		initC:          make(chan struct{}),
		firstTimeInitC: make(chan struct{}),
	}
	t.Cleanup(func() {
		cacheHealthReporter.remove(target, newCache.id)
		cacheHealthReporter.remove(target, oldCache.id)
	})

	// The new cache initializes and becomes healthy.
	newCache.setInitError(nil)
	// The old cache fails on the way down (context cancelled/watcher closed)
	// and writes an unhealthy value, then stops forever.
	oldCache.setInitError(trace.Errorf("shutting down"))

	// The new cache is still running and healthy, so the metric must report
	// healthy (1). On the buggy implementation it was stuck at 0.
	v, ok := readGauge(t, cacheHealth, target)
	require.True(t, ok)
	require.Equal(t, 1.0, v)
}

// TestHealthReporter_ConcurrentOverlap exercises the reporter the way the
// bug manifests in production: many cache instances sharing a target start and
// stop concurrently. It runs under -race to catch data races, and asserts that
// once every instance has stopped the series is removed (never stuck).
func TestHealthReporter_ConcurrentOverlap(t *testing.T) {
	g := newTestHealthGauge()
	r := newHealthReporter(g)

	const target = "auth"
	const instances = 50

	var wg sync.WaitGroup
	for i := 1; i <= instances; i++ {
		wg.Add(1)
		go func(id uint64) {
			defer wg.Done()
			// Come up healthy, flap, then shut down writing an unhealthy value
			// on the way out (as a real cache does when its watcher closes).
			r.setHealth(target, id, true)
			r.setHealth(target, id, false)
			r.setHealth(target, id, true)
			r.setHealth(target, id, false)
			r.remove(target, id)
		}(uint64(i))
	}
	wg.Wait()

	_, ok := readGauge(t, g, target)
	require.False(t, ok, "series must be removed once all instances have stopped")
}

func TestHealthReporter(t *testing.T) {
	const target = "auth"

	t.Run("single healthy instance reports healthy", func(t *testing.T) {
		g := newTestHealthGauge()
		r := newHealthReporter(g)

		r.setHealth(target, 1, true)

		v, ok := readGauge(t, g, target)
		require.True(t, ok)
		require.Equal(t, 1.0, v)
	})

	t.Run("single unhealthy instance reports unhealthy", func(t *testing.T) {
		g := newTestHealthGauge()
		r := newHealthReporter(g)

		r.setHealth(target, 1, false)

		v, ok := readGauge(t, g, target)
		require.True(t, ok)
		require.Equal(t, 0.0, v)
	})

	t.Run("healthy if any running instance is healthy", func(t *testing.T) {
		g := newTestHealthGauge()
		r := newHealthReporter(g)

		r.setHealth(target, 1, true)
		r.setHealth(target, 2, false)

		v, ok := readGauge(t, g, target)
		require.True(t, ok)
		require.Equal(t, 1.0, v, "one healthy instance should keep the target healthy")
	})

	t.Run("unhealthy shutdown does not clobber a healthy instance", func(t *testing.T) {
		g := newTestHealthGauge()
		r := newHealthReporter(g)

		// New instance healthy, old instance writes unhealthy on the way down.
		r.setHealth(target, 1, true)
		r.setHealth(target, 2, false)
		// Old instance stops; only the healthy instance remains.
		r.remove(target, 2)

		v, ok := readGauge(t, g, target)
		require.True(t, ok)
		require.Equal(t, 1.0, v)
	})

	t.Run("value recomputes from remaining instances after removal", func(t *testing.T) {
		g := newTestHealthGauge()
		r := newHealthReporter(g)

		// Only-healthy instance is the one being removed; the survivor is
		// unhealthy, so the reported value must drop to unhealthy.
		r.setHealth(target, 1, true)
		r.setHealth(target, 2, false)
		r.remove(target, 1)

		v, ok := readGauge(t, g, target)
		require.True(t, ok)
		require.Equal(t, 0.0, v)
	})

	t.Run("series is deleted when the last instance is removed", func(t *testing.T) {
		g := newTestHealthGauge()
		r := newHealthReporter(g)

		r.setHealth(target, 1, false)
		_, ok := readGauge(t, g, target)
		require.True(t, ok)

		r.remove(target, 1)

		_, ok = readGauge(t, g, target)
		require.False(t, ok, "the series should be removed once no instances remain")
	})

	t.Run("removing an unknown instance is a no-op", func(t *testing.T) {
		g := newTestHealthGauge()
		r := newHealthReporter(g)

		r.remove(target, 42)
		_, ok := readGauge(t, g, target)
		require.False(t, ok)

		// Removing twice must not resurrect or corrupt anything either.
		r.setHealth(target, 1, true)
		r.remove(target, 1)
		r.remove(target, 1)
		_, ok = readGauge(t, g, target)
		require.False(t, ok)
	})

	t.Run("targets are tracked independently", func(t *testing.T) {
		g := newTestHealthGauge()
		r := newHealthReporter(g)

		r.setHealth("auth", 1, true)
		r.setHealth("okta", 2, false)

		v, ok := readGauge(t, g, "auth")
		require.True(t, ok)
		require.Equal(t, 1.0, v)

		v, ok = readGauge(t, g, "okta")
		require.True(t, ok)
		require.Equal(t, 0.0, v)

		// Removing one target's instance leaves the other untouched.
		r.remove("okta", 2)
		v, ok = readGauge(t, g, "auth")
		require.True(t, ok)
		require.Equal(t, 1.0, v)
		_, ok = readGauge(t, g, "okta")
		require.False(t, ok)
	})
}
