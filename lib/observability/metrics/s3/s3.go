// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
