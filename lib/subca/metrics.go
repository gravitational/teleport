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

package subca

import (
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/observability/metrics"
)

const metricSubsystem = "ca"

var (
	overrideReadHist = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: metricSubsystem,
		Name:      "override_read_seconds",
		Help:      "Measures the impact of reading and parsing CA overrides",
		Buckets:   prometheus.DefBuckets,
	}, []string{"ca_type", "result"})

	overrideApplyHist = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: metricSubsystem,
		Name:      "override_apply_seconds",
		Help:      "Measures the impact of applying certificate overrides to CA certificates",
		Buckets:   prometheus.DefBuckets,
	}, []string{"ca_type", "num_certificates", "result"})
)

// RegisterMetrics registers subca prometheus metrics.
func RegisterMetrics(registerer prometheus.Registerer) error {
	return trace.Wrap(metrics.RegisterCollectors(registerer,
		overrideReadHist,
		overrideApplyHist,
	))
}

func resultFromError(err error) string {
	if err == nil {
		return "success"
	}
	return "error"
}
