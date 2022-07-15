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

package dynamo

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	apiRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dynamo_api_total",
			Help: "Number of requests to the dynamo api",
		},
		[]string{"type", "operation"},
	)
	apiRequestsFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dynamo_api_failed_total",
			Help: "Number of failed requests to the dynamo api",
		},
		[]string{"type", "operation"},
	)
	apiRequestLatencies = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "dynamo_api_request_seconds",
			Help: "Request latency of the dynamo api",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
		[]string{"type", "operation"},
	)

	dynamoCollectors = []prometheus.Collector{
		apiRequests,
		apiRequestsFailed,
		apiRequestLatencies,
	}
)

// TableType indicates which type of table metrics are being calculated for
type TableType string

const (
	// Backend is a table used to store backend data.
	Backend TableType = "backend"
	// Events is a table used to store audit events.
	Events TableType = "events"
)

// recordMetrics updates the set of dynamo api metrics
func recordMetrics(tableType TableType, operation string, err error, latency float64) {
	labels := []string{string(tableType), operation}
	apiRequestLatencies.WithLabelValues(labels...).Observe(latency)
	apiRequests.WithLabelValues(labels...).Inc()
	if err != nil {
		apiRequestsFailed.WithLabelValues(labels...).Inc()
	}
}
