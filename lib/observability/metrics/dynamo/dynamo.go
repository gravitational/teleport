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

package dynamo

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	apiRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dynamo_requests_total",
			Help: "Total number of requests to the DynamoDB API",
		},
		[]string{"type", "operation"},
	)
	apiRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dynamo_requests",
			Help: "Number of failed requests to the DynamoDB API by result",
		},
		[]string{"type", "operation", "result"},
	)
	apiRequestLatencies = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "dynamo_requests_seconds",
			Help: "Request latency for the DynamoDB API",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
		[]string{"type", "operation"},
	)

	dynamoCollectors = []prometheus.Collector{
		apiRequests,
		apiRequestsTotal,
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
	apiRequestsTotal.WithLabelValues(labels...).Inc()
	apiRequestLatencies.WithLabelValues(labels...).Observe(latency)

	result := "success"
	if err != nil {
		result = "error"
	}
	apiRequests.WithLabelValues(append(labels, result)...).Inc()
}
