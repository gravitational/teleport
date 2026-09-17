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

package auditlogv1

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/observability/metrics"
)

const metricsSubsystem = "audit_queue"

var BatchSubmissionsAcceptedMetric = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: teleport.MetricNamespace,
	Subsystem: metricsSubsystem,
	Name:      "batch_submissions_accepted_total",
	Help:      "Sealed audit queue batches that obtained a decrypt slot",
})

var BatchSubmissionsRejectedMetric = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: teleport.MetricNamespace,
	Subsystem: metricsSubsystem,
	Name:      "batch_submissions_rejected_total",
	Help:      "Sealed audit queue batches rejected because no decrypt slot freed within the wait timeout",
})

var DecryptSlotWaitMetric = prometheus.NewHistogram(prometheus.HistogramOpts{
	Namespace: teleport.MetricNamespace,
	Subsystem: metricsSubsystem,
	Name:      "decrypt_slot_wait_seconds",
	Help:      "Time a submission waited for a decrypt slot",
	Buckets:   prometheus.ExponentialBuckets(0.001, 2, 14),
})

func init() {
	metrics.RegisterPrometheusCollectors(
		BatchSubmissionsAcceptedMetric,
		BatchSubmissionsRejectedMetric,
		DecryptSlotWaitMetric,
	)
}
