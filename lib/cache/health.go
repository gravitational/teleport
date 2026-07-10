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
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
)

// cacheIDCounter hands out process-unique identifiers for cache instances so
// that the healthReporter can track the health of individual caches that share
// the same target (component name).
var cacheIDCounter atomic.Uint64

// nextCacheID returns a new process-unique cache identifier.
func nextCacheID() uint64 {
	return cacheIDCounter.Add(1)
}

// healthReporter maintains the teleport_cache_health gauge for every cache
// target. Because the gauge is keyed only by the cache target (component name),
// multiple concurrently-running cache instances with the same target would
// otherwise clobber each other's value on the shared series. This is possible
// whenever more than one instance of a cache is alive at the same time, for
// example:
//
//   - The Okta cache is started and stopped with the Okta plugin, which is
//     restarted (new instance started before the old one is fully stopped) on a
//     host CA rotation that rotates credentials.
//   - Toggling External Audit Storage in Teleport Cloud starts a new
//     TeleportProcess before the old one is shut down, so the old and new caches
//     of each type overlap.
//
// In those windows a shutting-down instance could write an unhealthy (0) value
// after a healthy new instance wrote 1, leaving the series stuck at 0 until the
// running cache happened to re-initialize.
//
// healthReporter fixes this by tracking each instance's health individually and
// deriving the reported value from the instances that are still running: the
// target is reported healthy if any running instance is healthy, unhealthy if
// all running instances are unhealthy, and the series is removed entirely once
// no instances remain.
type healthReporter struct {
	gauge *prometheus.GaugeVec

	mu sync.Mutex
	// health maps a cache target to the health of each running cache instance
	// (keyed by the instance's unique id) for that target.
	health map[string]map[uint64]bool
}

// newHealthReporter returns a healthReporter that maintains the provided gauge.
func newHealthReporter(gauge *prometheus.GaugeVec) *healthReporter {
	return &healthReporter{
		gauge:  gauge,
		health: make(map[string]map[uint64]bool),
	}
}

// setHealth records the health of the cache instance identified by id for the
// given target and updates the reported gauge value accordingly.
func (r *healthReporter) setHealth(target string, id uint64, healthy bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	instances := r.health[target]
	if instances == nil {
		instances = make(map[uint64]bool)
		r.health[target] = instances
	}
	instances[id] = healthy
	r.update(target)
}

// remove stops the cache instance identified by id from contributing to the
// gauge value for target. When the last instance for a target is removed the
// series is deleted so that the exporter stops reporting a value for a target
// that is no longer running (rather than leaving a stale one behind).
func (r *healthReporter) remove(target string, id uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	instances := r.health[target]
	if instances == nil {
		return
	}
	delete(instances, id)
	if len(instances) == 0 {
		delete(r.health, target)
	}
	r.update(target)
}

// update recomputes and publishes the gauge value for target. It must be called
// with r.mu held.
func (r *healthReporter) update(target string) {
	instances := r.health[target]
	if len(instances) == 0 {
		// No running instances: stop reporting for this target entirely so a
		// stale value can't linger on the exporter.
		r.gauge.DeleteLabelValues(target)
		return
	}

	// The target is healthy if any running instance is healthy.
	healthy := false
	for _, ok := range instances {
		if ok {
			healthy = true
			break
		}
	}

	if healthy {
		r.gauge.WithLabelValues(target).Set(1)
	} else {
		r.gauge.WithLabelValues(target).Set(0)
	}
}
