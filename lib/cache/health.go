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

	"github.com/prometheus/client_golang/prometheus"
)

// healthReporter aggregates the health of every open cache sharing a target
// label and publishes the combined value to a gauge. Writing the gauge
// directly from each cache is racy: two caches with the same target clobber
// each other's writes, and a closed cache leaves its last value behind.
//
// The aggregation is modeled and verified in formalmethods/cachehealth (the
// Metric machine): a target is healthy if it has no open caches, or if at
// least one of its open caches is healthy.
type healthReporter struct {
	gauge *prometheus.GaugeVec

	mu sync.Mutex
	// health holds the last reported health of every open cache, keyed by
	// target label.
	health map[string]map[*Cache]bool
}

func newHealthReporter(gauge *prometheus.GaugeVec) *healthReporter {
	return &healthReporter{
		gauge:  gauge,
		health: make(map[string]map[*Cache]bool),
	}
}

// report records the health of a cache and republishes the combined health of
// its target. Reports from a closed cache are ignored: Close sets c.closed
// before deregistering, so once deregister has run, any late report (e.g. the
// update loop publishing its shutdown error) observes the flag here and
// cannot resurrect the cache's entry.
func (r *healthReporter) report(c *Cache, healthy bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if c.closed.Load() {
		return
	}
	caches := r.health[c.Config.target]
	if caches == nil {
		caches = make(map[*Cache]bool)
		r.health[c.Config.target] = caches
	}
	caches[c] = healthy
	r.publishLocked(c.Config.target)
}

// deregister removes a closed cache from its target's health aggregation and
// republishes the combined health of the remaining caches.
func (r *healthReporter) deregister(c *Cache) {
	r.mu.Lock()
	defer r.mu.Unlock()

	caches := r.health[c.Config.target]
	delete(caches, c)
	if len(caches) == 0 {
		delete(r.health, c.Config.target)
	}
	r.publishLocked(c.Config.target)
}

func (r *healthReporter) publishLocked(target string) {
	value := 0.0
	if r.anyHealthyLocked(target) {
		value = 1.0
	}
	r.gauge.WithLabelValues(target).Set(value)
}

// anyHealthyLocked mirrors Metric.anyHealthy in the P model: a target with no
// open caches is healthy; otherwise it is healthy if any open cache is.
func (r *healthReporter) anyHealthyLocked(target string) bool {
	caches := r.health[target]
	if len(caches) == 0 {
		return true
	}
	for _, healthy := range caches {
		if healthy {
			return true
		}
	}
	return false
}
