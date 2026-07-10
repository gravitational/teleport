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

package cache

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
)

// newMetricTestCache builds a minimal Cache that is only wired up enough to
// exercise the metric-reporting code paths (setInitError, deleteMetrics). It
// deliberately avoids New() so the test can create many instances cheaply and
// control their identity precisely.
func newMetricTestCache(target, metricComponent string) *Cache {
	return &Cache{
		Config: Config{
			target:          target,
			MetricComponent: metricComponent,
		},
		initC:          make(chan struct{}),
		firstTimeInitC: make(chan struct{}),
	}
}

// collectCacheHealth returns the value of every teleport_cache_health series
// whose cache_component label matches target.
func collectCacheHealth(t *testing.T, target string) []float64 {
	t.Helper()

	reg := prometheus.NewRegistry()
	require.NoError(t, reg.Register(cacheHealth))

	families, err := reg.Gather()
	require.NoError(t, err)

	var values []float64
	for _, family := range families {
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == teleport.TagCacheComponent && label.GetValue() == target {
					values = append(values, metric.GetGauge().GetValue())
				}
			}
		}
	}
	return values
}

// TestCacheHealthMetricPerInstance ensures that two caches which share a target
// (as happens in production, where a proxy runs one "remote-proxy" cache per
// trusted cluster) do not clobber each other's health metric. Each cache must
// report its own health under its own series.
func TestCacheHealthMetricPerInstance(t *testing.T) {
	// Use a target unique to this test so the assertions are not affected by
	// caches created elsewhere in the process.
	const target = "test-remote-proxy-clobber"

	healthy := newMetricTestCache(target, "reverse:leaf-a:cache")
	unhealthy := newMetricTestCache(target, "reverse:leaf-b:cache")
	t.Cleanup(func() {
		healthy.deleteMetrics()
		unhealthy.deleteMetrics()
	})

	// One cache initializes successfully, the other fails. They share a target.
	healthy.setInitError(nil)
	unhealthy.setInitError(trace.Errorf("degraded backend"))

	values := collectCacheHealth(t, target)
	require.ElementsMatch(t, []float64{1, 0}, values,
		"each cache sharing a target must report its own health; a missing or "+
			"clobbered value means one cache overwrote the other's series")
}

// TestCacheHealthMetricDeletedOnClose ensures a cache's health series is removed
// when the cache is torn down, so transient caches (e.g. per trusted cluster)
// do not leak stale series.
func TestCacheHealthMetricDeletedOnClose(t *testing.T) {
	const target = "test-remote-proxy-close"

	c := newMetricTestCache(target, "reverse:leaf-c:cache")
	c.setInitError(nil)
	require.Equal(t, []float64{1}, collectCacheHealth(t, target))

	c.deleteMetrics()
	require.Empty(t, collectCacheHealth(t, target),
		"cache health series should be removed once the cache is closed")
}
