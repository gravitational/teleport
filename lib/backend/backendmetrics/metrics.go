// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package backendmetrics

import (
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/observability/metrics"
)

var (
	Requests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: teleport.MetricBackendRequests,
			Help: "Number of requests to the backend (reads, writes, and keepalives)",
		},
		[]string{teleport.ComponentLabel, teleport.TagReq, teleport.TagRange},
	)
	Watchers = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: teleport.MetricBackendWatchers,
			Help: "Number of active backend watchers",
		},
		[]string{teleport.ComponentLabel},
	)
	WatcherQueues = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: teleport.MetricBackendWatcherQueues,
			Help: "Watcher queue sizes",
		},
		[]string{teleport.ComponentLabel},
	)
	WriteRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: teleport.MetricBackendWriteRequests,
			Help: "Number of write requests to the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	Writes = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricBackendWrites,
			Help:      "Number of individual items written to the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	WriteRequestsFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: teleport.MetricBackendWriteFailedRequests,
			Help: "Number of failed write requests to the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	WriteRequestsFailedPrecondition = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricBackendWriteFailedPreconditionRequests,
			Help:      "Number of write requests that failed due to a precondition (existence, revision, value, etc)",
		},
		[]string{teleport.ComponentLabel},
	)
	AtomicWriteRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricBackendAtomicWriteRequests,
			Help:      "Number of atomic write requests to the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	AtomicWriteRequestsFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricBackendAtomicWriteFailedRequests,
			Help:      "Number of failed atomic write requests to the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	AtomicWriteConditionFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricBackendAtomicWriteConditionFailed,
			Help:      "Number of times an atomic write request results in condition failure",
		},
		[]string{teleport.ComponentLabel},
	)
	AtomicWriteLatencies = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricBackendAtomicWriteHistogram,
			Help:      "Latency for backend atomic write operations",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
		[]string{teleport.ComponentLabel},
	)
	AtomicWriteSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricBackendAtomicWriteSize,
			Help:      "Atomic write batch size",
			// buckets of the form 1, 2, 4, 8, 16, etc...
			Buckets: prometheus.ExponentialBuckets(1, 2, 8),
		},
		[]string{teleport.ComponentLabel},
	)
	AtomicWriteContention = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricBackendAtomicWriteContention,
			Help:      "Number of times atomic write requests experience contention",
		},
		[]string{teleport.ComponentLabel},
	)
	BatchWriteRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: teleport.MetricBackendBatchWriteRequests,
			Help: "Number of batch write requests to the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	BatchWriteRequestsFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: teleport.MetricBackendBatchFailedWriteRequests,
			Help: "Number of failed write requests to the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	ReadRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: teleport.MetricBackendReadRequests,
			Help: "Number of read requests to the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	Reads = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricBackendReads,
			Help:      "Number of individual items read from the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	ReadRequestsFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: teleport.MetricBackendFailedReadRequests,
			Help: "Number of failed read requests to the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	StreamingRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: "backend",
			Name:      "stream_requests",
			Help:      "Number of inflight stream requests to the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	StreamingRequestsFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: "backend",
			Name:      "stream_requests_failed",
			Help:      "Number of failed stream requests to the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	BatchReadRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: teleport.MetricBackendBatchReadRequests,
			Help: "Number of read requests to the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	BatchReadRequestsFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: teleport.MetricBackendBatchFailedReadRequests,
			Help: "Number of failed read requests to the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	WriteLatencies = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: teleport.MetricBackendWriteHistogram,
			Help: "Latency for backend write operations",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
		[]string{teleport.ComponentLabel},
	)
	BatchWriteLatencies = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: teleport.MetricBackendBatchWriteHistogram,
			Help: "Latency for backend batch write operations",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
		[]string{teleport.ComponentLabel},
	)
	BatchReadLatencies = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: teleport.MetricBackendBatchReadHistogram,
			Help: "Latency for batch read operations",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
		[]string{teleport.ComponentLabel},
	)
	ReadLatencies = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: teleport.MetricBackendReadHistogram,
			Help: "Latency for read operations",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
		[]string{teleport.ComponentLabel},
	)
)

// RegisterCollectors ensures all backend metrics are registered
// with the provided [prometheus.Registerer].
func RegisterCollectors(reg prometheus.Registerer) error {
	return trace.Wrap(metrics.RegisterCollectors(reg,
		Watchers, WatcherQueues, Requests, WriteRequests,
		WriteRequestsFailed, BatchWriteRequests, BatchWriteRequestsFailed, ReadRequests,
		ReadRequestsFailed, BatchReadRequests, BatchReadRequestsFailed, WriteLatencies,
		WriteRequestsFailedPrecondition,
		AtomicWriteRequests, AtomicWriteRequestsFailed, AtomicWriteConditionFailed, AtomicWriteLatencies,
		AtomicWriteContention, AtomicWriteSize, Reads, Writes,
		BatchWriteLatencies, BatchReadLatencies, ReadLatencies,
		StreamingRequests, StreamingRequestsFailed,
	))
}
