// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// newHealthTestCache builds the minimal Cache needed to exercise the health
// reporting code paths (setInitError / deregisterHealth) without standing up a
// real backend.
func newHealthTestCache(target string) *Cache {
	return &Cache{
		Config:         Config{target: target},
		initC:          make(chan struct{}),
		firstTimeInitC: make(chan struct{}),
	}
}

// TestCacheHealthMetric_MultipleInstancesSameTarget reproduces the race in
// which two caches sharing a metric target clobber each other's value in the
// shared cacheHealth gauge.
//
// This mirrors the P model in formalmethods/cachehealth: a single Metric with
// two Cache instances reporting to it. The metric must report healthy if any
// of the caches reporting to it is healthy (System.p's anyHealthy()).
func TestCacheHealthMetric_MultipleInstancesSameTarget(t *testing.T) {
	const target = "test-cache-health-clobber"
	cacheHealth.DeleteLabelValues(target)
	t.Cleanup(func() { cacheHealth.DeleteLabelValues(target) })

	a := newHealthTestCache(target)
	b := newHealthTestCache(target)

	// Cache a comes up healthy.
	a.setInitError(nil)
	require.Equal(t, 1.0, testutil.ToFloat64(cacheHealth.WithLabelValues(target)),
		"a healthy => target healthy")

	// Cache b then reports unhealthy. Under the buggy implementation this
	// clobbers a's healthy value even though a is still healthy.
	b.setInitError(trace.Errorf("b failed to initialize"))
	require.Equal(t, 1.0, testutil.ToFloat64(cacheHealth.WithLabelValues(target)),
		"a still healthy => target must remain healthy despite b being unhealthy")

	// Both unhealthy => target unhealthy.
	a.setInitError(trace.Errorf("a failed to initialize"))
	require.Equal(t, 0.0, testutil.ToFloat64(cacheHealth.WithLabelValues(target)),
		"all caches unhealthy => target unhealthy")

	// a recovers => target healthy again.
	a.setInitError(nil)
	require.Equal(t, 1.0, testutil.ToFloat64(cacheHealth.WithLabelValues(target)),
		"a recovered => target healthy")
}

// TestCacheHealthMetric_DeregisterOnClose verifies that a closed/deregistered
// cache no longer contributes its (possibly stale) value to the shared gauge.
//
// This mirrors the eDeregister transition in System.p: when a Cache stops it
// removes itself from the Metric, and the metric re-derives anyHealthy().
func TestCacheHealthMetric_DeregisterOnClose(t *testing.T) {
	const target = "test-cache-health-deregister"
	cacheHealth.DeleteLabelValues(target)
	t.Cleanup(func() { cacheHealth.DeleteLabelValues(target) })

	a := newHealthTestCache(target)
	b := newHealthTestCache(target)

	a.setInitError(nil)                         // a healthy
	b.setInitError(trace.Errorf("b unhealthy")) // b unhealthy
	require.Equal(t, 1.0, testutil.ToFloat64(cacheHealth.WithLabelValues(target)))

	// Now a goes unhealthy: only unhealthy caches remain => unhealthy.
	a.setInitError(trace.Errorf("a unhealthy"))
	require.Equal(t, 0.0, testutil.ToFloat64(cacheHealth.WithLabelValues(target)))

	// b deregisters (closes). Only a remains and it is unhealthy => still 0.
	b.deregisterHealth()
	require.Equal(t, 0.0, testutil.ToFloat64(cacheHealth.WithLabelValues(target)))

	// a deregisters too. With no caches reporting, the target is healthy
	// (a valid state per System.p's anyHealthy(): an empty metric is healthy).
	a.deregisterHealth()
	require.Equal(t, 1.0, testutil.ToFloat64(cacheHealth.WithLabelValues(target)))

	// Deregistering an unknown/already-removed cache is a no-op.
	a.deregisterHealth()
	require.Equal(t, 1.0, testutil.ToFloat64(cacheHealth.WithLabelValues(target)))
}

// healthStateMachine is a rapid state machine that mirrors the P model in
// formalmethods/cachehealth: a single Metric (target) with an arbitrary number
// of Cache instances reporting to it. Actions correspond to the events driven
// by Driver.p / FaultInjector.p:
//
//   - Report    => a cache reports its (possibly changed) health (eSetHealth /
//     eReport). This also (re)registers a previously stopped cache.
//   - Deregister => a cache stops (eStop => eDeregister).
//
// Check enforces the invariant from Spec.p (MetricEventuallyConverges): the
// gauge must always equal expected(), i.e. healthy iff no caches are up or at
// least one up cache is healthy. Because every action updates the gauge
// synchronously, the model is always converged, which is a strictly stronger
// property than the "eventually converges" liveness property in Spec.p.
type healthStateMachine struct {
	tracker *cacheHealthTracker
	gauge   *prometheus.GaugeVec
	target  string

	// caches is every cache instance created so far.
	caches []*Cache
	// up is the set of caches currently registered to the target (analogous to
	// Spec.p's `up`).
	up map[*Cache]struct{}
	// healthyUp is the subset of up caches currently reporting healthy
	// (analogous to Spec.p's `healthyUp`).
	healthyUp map[*Cache]struct{}
	// reported is true once the target's gauge has been written at least once.
	// Before that the metric is undefined, matching System.p where the Metric
	// announces nothing until its first event.
	reported bool
}

// Report models a cache reporting its health (FaultInjector's eSetHealth
// followed by the cache's eReport). It may target an existing cache or spin up
// a new one, and it re-registers a cache that had previously stopped.
func (m *healthStateMachine) Report(t *rapid.T) {
	idx := rapid.IntRange(0, len(m.caches)).Draw(t, "cache")
	var c *Cache
	if idx == len(m.caches) {
		c = &Cache{Config: Config{target: m.target}}
		m.caches = append(m.caches, c)
	} else {
		c = m.caches[idx]
	}

	healthy := rapid.Bool().Draw(t, "healthy")
	m.tracker.setHealth(c, healthy)
	m.reported = true

	m.up[c] = struct{}{}
	if healthy {
		m.healthyUp[c] = struct{}{}
	} else {
		delete(m.healthyUp, c)
	}
}

// Deregister models a cache stopping (eStop => eDeregister).
func (m *healthStateMachine) Deregister(t *rapid.T) {
	if len(m.caches) == 0 {
		t.Skip("no caches to deregister")
	}
	c := m.caches[rapid.IntRange(0, len(m.caches)-1).Draw(t, "cache")]

	m.tracker.deregister(c)
	m.reported = true

	delete(m.up, c)
	delete(m.healthyUp, c)
}

// Check is the invariant from Spec.p: the reported metric must equal
// expected() = (no caches up) || (at least one up cache healthy).
func (m *healthStateMachine) Check(t *rapid.T) {
	if !m.reported {
		return
	}

	got := testutil.ToFloat64(m.gauge.WithLabelValues(m.target))
	want := 0.0
	if len(m.up) == 0 || len(m.healthyUp) > 0 {
		want = 1.0
	}
	require.Equal(t, want, got,
		"metric diverged: up=%d healthyUp=%d", len(m.up), len(m.healthyUp))
}

// TestProperty_CacheHealthMetric_Converges is the property-based analog of the
// tcFix test in formalmethods/cachehealth/PTst/Tests.p. It drives random
// sequences of health reports and deregistrations across many cache instances
// sharing a single target and asserts the aggregate gauge never diverges from
// the expected value.
func TestProperty_CacheHealthMetric_Converges(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		gauge := prometheus.NewGaugeVec(
			prometheus.GaugeOpts{Name: "test_cache_health"},
			[]string{"target"},
		)
		m := &healthStateMachine{
			tracker:   newCacheHealthTracker(gauge),
			gauge:     gauge,
			target:    "test",
			up:        make(map[*Cache]struct{}),
			healthyUp: make(map[*Cache]struct{}),
		}
		t.Repeat(rapid.StateMachineActions(m))
	})
}
