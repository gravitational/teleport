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
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// The tests in this file are the Go analog of the P model in
// formalmethods/cachehealth. Multiple caches sharing the same target label
// report to the same health metric, and the MetricEventuallyConverges spec
// requires that the metric converges to:
//
//	healthy == (no cache is up) || (at least one up cache is healthy)
//
// The model events map to the implementation as follows:
//
//	eReport     -> Cache.setInitError (healthy iff err == nil)
//	eStop       -> Cache.Close
//	eDeregister -> performed by Cache.Close on behalf of the cache
//
// healthTestTargetCounter distinguishes gauge label values across tests and
// property-test iterations, since the cacheHealth gauge is package-global.
var healthTestTargetCounter atomic.Uint64

func newHealthTestTarget(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, healthTestTargetCounter.Add(1))
}

// newHealthTestCache builds the minimal Cache needed to exercise setInitError
// and Close, the two paths that publish the cache health metric.
func newHealthTestCache(target string) *Cache {
	return &Cache{
		Config:         Config{target: target},
		initC:          make(chan struct{}),
		firstTimeInitC: make(chan struct{}),
		cancel:         func() {},
		eventsFanout:   services.NewFanoutV2(services.FanoutV2Config{}),
		lowVolumeEventsFanout: utils.NewRoundRobin([]*services.FanoutV2{
			services.NewFanoutV2(services.FanoutV2Config{}),
		}),
	}
}

func cacheHealthValue(target string) float64 {
	return testutil.ToFloat64(cacheHealth.WithLabelValues(target))
}

// TestCacheHealthMetricAggregatesInstances verifies that an unhealthy cache
// cannot clobber the report of a healthy cache sharing the same target,
// regardless of report order.
func TestCacheHealthMetricAggregatesInstances(t *testing.T) {
	target := newHealthTestTarget("aggregate")
	t.Cleanup(func() { cacheHealth.DeleteLabelValues(target) })

	a := newHealthTestCache(target)
	b := newHealthTestCache(target)

	// A healthy report followed by an unhealthy report from another cache
	// must not flip the metric to unhealthy.
	a.setInitError(nil)
	b.setInitError(errors.New("watcher failed"))
	require.Equal(t, 1.0, cacheHealthValue(target), "metric must stay healthy while cache a is healthy")

	// Reverse order: an unhealthy report first must not mask a later
	// healthy one.
	b.setInitError(errors.New("watcher failed"))
	a.setInitError(nil)
	require.Equal(t, 1.0, cacheHealthValue(target), "metric must report healthy while cache a is healthy")

	// Only once every cache is unhealthy may the metric report unhealthy.
	a.setInitError(errors.New("watcher failed"))
	require.Equal(t, 0.0, cacheHealthValue(target), "metric must report unhealthy when all caches are unhealthy")

	// A single recovery makes the target healthy again.
	b.setInitError(nil)
	require.Equal(t, 1.0, cacheHealthValue(target), "metric must report healthy after cache b recovers")
}

// TestCacheHealthMetricDeregisterOnClose verifies that a closed cache stops
// contributing to the health metric, and that a target with no remaining
// caches reports healthy (the model's anyHealthy on an empty map).
func TestCacheHealthMetricDeregisterOnClose(t *testing.T) {
	target := newHealthTestTarget("deregister")
	t.Cleanup(func() { cacheHealth.DeleteLabelValues(target) })

	a := newHealthTestCache(target)
	b := newHealthTestCache(target)

	a.setInitError(nil)
	b.setInitError(errors.New("watcher failed"))

	// Closing the healthy cache leaves only the unhealthy one.
	a.Close()
	require.Equal(t, 0.0, cacheHealthValue(target), "metric must report unhealthy when the only remaining cache is unhealthy")

	// Closing the last cache must not leave a stale unhealthy value behind.
	b.Close()
	require.Equal(t, 1.0, cacheHealthValue(target), "metric must report healthy once no caches remain")
}

// TestCacheHealthMetricIgnoresReportsAfterClose covers the race between
// Cache.Close and the final setInitError performed by the update loop during
// shutdown: reports from a closed cache must be ignored, mirroring the
// Stopped state of the Cache machine ignoring eSetHealth.
func TestCacheHealthMetricIgnoresReportsAfterClose(t *testing.T) {
	target := newHealthTestTarget("afterclose")
	t.Cleanup(func() { cacheHealth.DeleteLabelValues(target) })

	a := newHealthTestCache(target)
	b := newHealthTestCache(target)

	a.setInitError(nil)
	b.Close()
	// The update loop of b reports its shutdown error after Close won the
	// race; the report must not count against the target's health.
	b.setInitError(errors.New("watcher closed"))
	require.Equal(t, 1.0, cacheHealthValue(target), "reports from a closed cache must not affect the metric")
}

// TestCacheHealthMetricConcurrentReports exercises the reporting paths under
// the race detector: one goroutine plays cache b's update loop while another
// closes b, with cache a healthy throughout.
func TestCacheHealthMetricConcurrentReports(t *testing.T) {
	target := newHealthTestTarget("concurrent")
	t.Cleanup(func() { cacheHealth.DeleteLabelValues(target) })

	a := newHealthTestCache(target)
	b := newHealthTestCache(target)

	a.setInitError(nil)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := range 100 {
			if i%2 == 0 {
				b.setInitError(nil)
			} else {
				b.setInitError(errors.New("watcher failed"))
			}
		}
	}()
	go func() {
		defer wg.Done()
		b.Close()
	}()
	wg.Wait()

	// The update loop publishes one final error after losing the race
	// with Close.
	b.setInitError(errors.New("watcher closed"))
	require.Equal(t, 1.0, cacheHealthValue(target), "metric must report healthy while cache a is healthy")
}

// TestCacheHealthMetricConvergesProperty is the Go analog of the tcFix test
// in formalmethods/cachehealth: it drives a random sequence of health reports
// and stops across two caches sharing a target (Driver.p and FaultInjector)
// and checks the MetricEventuallyConverges invariant after every step
// (Spec.p).
func TestCacheHealthMetricConvergesProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		target := newHealthTestTarget("prop")
		defer cacheHealth.DeleteLabelValues(target)

		const n = 2
		var caches [n]*Cache
		for i := range caches {
			caches[i] = newHealthTestCache(target)
		}

		// Model state per Spec.p: up caches, their health, and stopped
		// caches whose reports must be ignored.
		var up, healthy, stopped [n]bool
		expectedHealthy := func() bool {
			anyUp, anyHealthyUp := false, false
			for i := range n {
				if up[i] {
					anyUp = true
					anyHealthyUp = anyHealthyUp || healthy[i]
				}
			}
			return !anyUp || anyHealthyUp
		}

		steps := rapid.IntRange(1, 24).Draw(rt, "steps")
		for step := range steps {
			i := rapid.IntRange(0, n-1).Draw(rt, "cache")
			action := rapid.SampledFrom([]string{"report_healthy", "report_unhealthy", "stop"}).Draw(rt, "action")
			switch action {
			case "report_healthy":
				caches[i].setInitError(nil)
				if !stopped[i] {
					up[i], healthy[i] = true, true
				}
			case "report_unhealthy":
				caches[i].setInitError(errors.New("watcher failed"))
				if !stopped[i] {
					up[i], healthy[i] = true, false
				}
			case "stop":
				caches[i].Close()
				stopped[i], up[i] = true, false
			}

			got := cacheHealthValue(target) == 1.0
			require.Equalf(rt, expectedHealthy(), got,
				"metric diverged from spec at step %d (action %s on cache %d)", step, action, i)
		}
	})
}
