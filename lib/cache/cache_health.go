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

var cacheHealthMetric = newCacheHealthTracker(cacheHealth)

// cacheHealthTracker aggregates the health of cache instances that share a
// metric target. It implements the Metric machine in
// formalmethods/cachehealth/PSrc/System.p.
type cacheHealthTracker struct {
	gauge *prometheus.GaugeVec

	mu     sync.Mutex
	health map[string]map[*cacheHealthReporter]bool
}

func newCacheHealthTracker(gauge *prometheus.GaugeVec) *cacheHealthTracker {
	return &cacheHealthTracker{
		gauge:  gauge,
		health: make(map[string]map[*cacheHealthReporter]bool),
	}
}

func (t *cacheHealthTracker) newReporter(target string) *cacheHealthReporter {
	return &cacheHealthReporter{
		tracker: t,
		target:  target,
	}
}

// cacheHealthReporter identifies one cache to a shared cacheHealthTracker.
// stopped is protected by tracker.mu so a report racing with close cannot
// register the cache again after it has been removed.
type cacheHealthReporter struct {
	tracker *cacheHealthTracker
	target  string
	stopped bool
}

// report records this cache's health and updates the aggregate gauge.
func (r *cacheHealthReporter) report(healthy bool) {
	if r == nil {
		return
	}

	r.tracker.mu.Lock()
	defer r.tracker.mu.Unlock()

	if r.stopped {
		return
	}
	instances := r.tracker.health[r.target]
	if instances == nil {
		instances = make(map[*cacheHealthReporter]bool)
		r.tracker.health[r.target] = instances
	}
	instances[r] = healthy
	r.tracker.updateGaugeLocked(r.target)
}

// close deregisters this cache and updates the aggregate gauge. A target with
// no remaining caches reports healthy, as specified by System.p.
func (r *cacheHealthReporter) close() {
	if r == nil {
		return
	}

	r.tracker.mu.Lock()
	defer r.tracker.mu.Unlock()

	if r.stopped {
		return
	}
	r.stopped = true

	instances := r.tracker.health[r.target]
	delete(instances, r)
	if len(instances) == 0 {
		delete(r.tracker.health, r.target)
	}
	r.tracker.updateGaugeLocked(r.target)
}

func (t *cacheHealthTracker) updateGaugeLocked(target string) {
	value := 0.0
	instances := t.health[target]
	if len(instances) == 0 {
		value = 1.0
	} else {
		for _, healthy := range instances {
			if healthy {
				value = 1.0
				break
			}
		}
	}
	t.gauge.WithLabelValues(target).Set(value)
}
