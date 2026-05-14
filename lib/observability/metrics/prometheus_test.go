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
package metrics

import (
	"slices"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func TestCollectorRegistration(t *testing.T) {
	// Create two metrics that will be registered with different
	// registries.
	metric1 := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "test",
		Subsystem: "metrics",
		Name:      "fake1",
		Help:      "not a real metric",
	}, []string{"test"})
	metric1.WithLabelValues("test").Add(1)

	metric2 := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "test",
		Subsystem: "metrics",
		Name:      "fake2",
		Help:      "not a real metric",
	}, []string{"test"})
	metric2.WithLabelValues("test").Add(1)

	// Validate that neither metric is present in the global registry.
	metrics, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	require.False(t, slices.ContainsFunc(metrics, func(m *dto.MetricFamily) bool {
		return m.Name != nil && (*m.Name == "test_metrics_fake1" || *m.Name == "test_metrics_fake2")
	}))

	// Validate that no metrics are present in the test registry.
	registry := prometheus.NewRegistry()
	metrics, err = registry.Gather()
	require.NoError(t, err)
	require.Empty(t, metrics)

	// Register metric1 with only the test registry.
	require.NoError(t, RegisterCollectors(registry, metric1))

	// Validate that metric1 is not present in the global registry.
	metrics, err = prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	require.False(t, slices.ContainsFunc(metrics, func(m *dto.MetricFamily) bool {
		return m.Name != nil && (*m.Name == "test_metrics_fake1" || *m.Name == "test_metrics_fake2")
	}))

	// Validate that metric1 is present in the test registry.
	metrics, err = registry.Gather()
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	require.True(t, slices.ContainsFunc(metrics, func(m *dto.MetricFamily) bool {
		return m.Name != nil && *m.Name == "test_metrics_fake1"
	}))

	// Register metric2 with only the global registry.
	require.NoError(t, RegisterPrometheusCollectors(metric2))
	// Remove the metric to prevent polluting the default registry so
	// that running with -count > 1 does not fail.
	t.Cleanup(func() { _ = prometheus.DefaultRegisterer.Unregister(metric2) })

	// Validate that metric2 is present in the global registry.
	metrics, err = prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	require.True(t, slices.ContainsFunc(metrics, func(m *dto.MetricFamily) bool {
		return m.Name != nil && *m.Name == "test_metrics_fake2"
	}))

	// Validate that metric2 is not present in the test registry.
	metrics, err = registry.Gather()
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	require.False(t, slices.ContainsFunc(metrics, func(m *dto.MetricFamily) bool {
		return m.Name != nil && *m.Name == "test_metrics_fake2"
	}))

	// Validate that registering the same metric twice doesn't produce an error.
	require.NoError(t, RegisterCollectors(registry, metric1))
	require.NoError(t, RegisterPrometheusCollectors(metric2))
}
