/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package s3

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	apiRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "s3_requests_total",
			Help: "Total number of requests to the S3 API",
		},
		[]string{"operation"},
	)
	apiRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "s3_requests",
			Help: "Number of requests to the AS3 API by result",
		},
		[]string{"operation", "result"},
	)
	apiRequestLatencies = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "s3_requests_seconds",
			Help: "Request latency for the S3 API",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
		[]string{"operation"},
	)

	s3Collectors = []prometheus.Collector{
		apiRequestsTotal,
		apiRequests,
		apiRequestLatencies,
	}
)

// recordMetrics updates the set of s3 api metrics
func recordMetrics(operation string, err error, latency float64) {
	apiRequestLatencies.WithLabelValues(operation).Observe(latency)
	apiRequestsTotal.WithLabelValues(operation).Inc()

	result := "success"
	if err != nil {
		result = "error"
	}
	apiRequests.WithLabelValues(operation, result).Inc()
}
