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

// TODO(russjones): Write comment. Think of a better name?
type HealthReporter struct {
	mu sync.Mutex

	// gauge is the underlying Prometheus metric that is emitted.
	gauge *prometheus.GaugeVec
	// health maps a caches target to an instance of a cache to the health of a
	// cache. A single HealthReporter is created for a TeleportProcess which means
	// caches for many different targets must be tracked. For example: okta,
	// auth, discovery.
	health map[string]map[*Cache]bool
}

func NewHealthReporter(registry *metrics.Registry) (*HealthReporter, error) {
	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: registry.Namespace(),
			Subsystem: "cache",
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
		health: make(map[string]map[*Cache]bool),
	}, nil
}

func (m *HealthReporter) Report(c *Cache, health bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// If Close() has been called on a cache, do no report any
	// additional health information.
	if c.closed.Load() {
		return
	}

	if m.health[c.target] == nil {
		m.health[c.target] = make(map[*Cache]bool)
	}
	m.health[c.target][c] = health

	m.gauge.WithLabelValues(c.target).Set(m.anyHealthy(c.target))
}

func (m *HealthReporter) Deregister(c *Cache) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.health[c.target] == nil {
		m.health[c.target] = make(map[*Cache]bool)
	}
	delete(m.health[c.target], c)

	m.gauge.WithLabelValues(c.target).Set(m.anyHealthy(c.target))
}

func (m *HealthReporter) anyHealthy(target string) float64 {
	// If no cache is up, report healthy. This is a valid state and
	// alternative to deleting this metric.
	if len(m.health[target]) == 0 {
		return 1.0
	}

	// If any cache is is healthy, report healthy status.
	for _, healthy := range m.health[target] {
		if healthy {
			return 1.0
		}
	}

	// If nothing is healthy, then report unhealthy.
	return 0
}
