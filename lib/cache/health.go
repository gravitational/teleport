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
	"sync"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/observability/metrics"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
)

// HealthReporter aggregates cache health by target so multiple Cache instances
// can share one GaugeVec without overwriting one another's reports.
type HealthReporter struct {
	mu sync.Mutex

	// gauge is the underlying Prometheus GaugeVec metric.
	gauge *prometheus.GaugeVec

	// health groups cache instances by target to allow every cache instance
	// within the same target to publish to one GaugeVec metric.
	health map[string]*targetHealth
}

type targetHealth struct {
	// live is a set that tracks caches that are still running.
	live map[*Cache]struct{}

	// healthy is a subset of live caches that are still running and have
	// reported themselves as healthy.
	healthy map[*Cache]struct{}
}

// value returns the value of the GaugeVec for this target. This call runs in
// the metric-emission path, so it must always be O(1).
func (h *targetHealth) value() float64 {
	// A target remains healthy while at least one live cache is healthy, so an
	// unhealthy cache that is restarting cannot clobber the shared gauge.
	if len(h.healthy) > 0 {
		return 1.0
	}

	// Live caches exist but none are healthy, report the target as unhealthy.
	return 0
}

// NewHealthReporter returns a new HealthReporter whose GaugeVec has been
// registered with the registry.
func NewHealthReporter(registry *metrics.Registry) (*HealthReporter, error) {
	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: registry.Namespace(),
			Subsystem: registry.Subsystem(),
			Name:      "health",
			Help:      "Whether the cache for a particular Teleport service is healthy.",
		},
		[]string{teleport.TagCacheComponent},
	)
	if err := metrics.RegisterCollectors(registry, gauge); err != nil {
		return nil, trace.Wrap(err)
	}

	return &HealthReporter{
		gauge:  gauge,
		health: make(map[string]*targetHealth),
	}, nil
}

// Report the health of a cache. This call runs in the metric-emission path,
// so it must always be O(1).
func (m *HealthReporter) Report(c *Cache, health bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ignore reports racing with Close so a late report cannot undo Deregister.
	if c.closed.Load() {
		return
	}

	// Get (or create) target for specific service, for example: "auth", "okta".
	target, ok := m.health[c.target]
	if !ok {
		target = &targetHealth{
			live:    make(map[*Cache]struct{}),
			healthy: make(map[*Cache]struct{}),
		}
		m.health[c.target] = target
	}

	// If a cache has reported it's health it must be live. Update healthy
	// set depending on what was reported.
	target.live[c] = struct{}{}
	if health {
		target.healthy[c] = struct{}{}
	} else {
		delete(target.healthy, c)
	}

	m.gauge.WithLabelValues(c.target).Set(target.value())
}

// Deregister a cache from the health reporter. Should happen upon close of a
// cache. This call runs in the metric-emission path, so it must always be O(1).
func (m *HealthReporter) Deregister(c *Cache) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get target for specific service, for example: "auth", "okta", etc.
	target, ok := m.health[c.target]
	if !ok {
		return
	}

	// Remove the cache from both sets because a deregistered cache is neither
	// healthy or live.
	delete(target.live, c)
	delete(target.healthy, c)

	// If the last live cache deregistered, remove the GaugeVec, otherwise
	// update its value.
	if len(target.live) == 0 {
		delete(m.health, c.target)
		m.gauge.DeleteLabelValues(c.target)
	} else {
		m.gauge.WithLabelValues(c.target).Set(target.value())
	}
}
