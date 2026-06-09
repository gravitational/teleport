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

package auditqueue

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
)

var batchSize = prometheus.NewHistogram(
	prometheus.HistogramOpts{
		Namespace: teleport.MetricNamespace,
		Name:      "audit_queue_batch_size",
		Help:      "Number of events committed in each batch.",
		Buckets:   []float64{1, 2, 5, 10, 25, 50, 100, 250},
	},
)

var orphansAdopted = prometheus.NewCounter(
	prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Name:      "audit_queue_orphans_adopted_total",
		Help:      "Total number of orphaned audit queue directories adopted, drained, and removed.",
	},
)

var orphanScanErrors = prometheus.NewCounter(
	prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Name:      "audit_queue_orphan_scan_errors_total",
		Help:      "Total number of errors encountered while scanning or draining orphaned audit-queue directories.",
	},
)

var softLimitWarnings = prometheus.NewCounter(
	prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Name:      "audit_queue_soft_limit_warnings_total",
		Help:      "Total number of soft limit warnings emitted because queue.db exceeded the configured soft limit.",
	},
)

var eventsEnqueued = prometheus.NewCounter(
	prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Name:      "audit_queue_events_enqueued_total",
		Help:      "Total number of audit events enqueued.",
	},
)

var eventsDelivered = prometheus.NewCounter(
	prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Name:      "audit_queue_events_delivered_total",
		Help:      "Total number of audit events successfully delivered.",
	},
)

var prometheusCollectors = []prometheus.Collector{
	batchSize,
	orphansAdopted,
	orphanScanErrors,
	softLimitWarnings,
	eventsEnqueued,
	eventsDelivered,
}
