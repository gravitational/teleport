/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package pgevents

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
)

var (
	statusLabel   = "status"
	successLabels = map[string]string{statusLabel: "success"}
	failureLabels = map[string]string{statusLabel: "failure"}
	labels        = []string{statusLabel}

	writeRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "postgres_events_backend_write_requests",
			Help:      "Number of write requests to postgres events, labeled with the request `status` (success or failure).",
		},
		labels,
	)
	batchReadRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "postgres_events_backend_batch_read_requests",
			Help:      "Number of batch read requests to postgres events, labeled with the request `status` (success or failure).",
		},
		labels,
	)
	batchDeleteRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "postgres_events_backend_batch_delete_requests",
			Help:      "Number of batch delete requests to postgres events, labeled with the request `status` (success or failure).",
		},
		labels,
	)
	writeLatencies = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "postgres_events_backend_write_seconds",
			Help:      "Latency for postgres events write operations, in seconds.",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
	)
	batchReadLatencies = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "postgres_events_backend_batch_read_seconds",
			Help:      "Latency for postgres events batch read operations, in seconds.",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
	)
	batchDeleteLatencies = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "postgres_events_backend_batch_delete_seconds",
			Help:      "Latency for postgres events cleanup operations, in seconds.",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^17 == 131 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 18),
		},
	)

	writeRequestsSuccess       = writeRequests.With(successLabels)
	writeRequestsFailure       = writeRequests.With(failureLabels)
	batchReadRequestsSuccess   = batchReadRequests.With(successLabels)
	batchReadRequestsFailure   = batchReadRequests.With(failureLabels)
	batchDeleteRequestsSuccess = batchDeleteRequests.With(successLabels)
	batchDeleteRequestsFailure = batchDeleteRequests.With(failureLabels)

	prometheusCollectors = []prometheus.Collector{
		writeRequests, batchReadRequests, batchDeleteRequests,
		writeLatencies, batchReadLatencies, batchDeleteLatencies,
	}
)
