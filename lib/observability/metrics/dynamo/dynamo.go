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
	"context"
	"time"

	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/smithy-go/middleware"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport/lib/observability/metrics"
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
			Help: "Number of requests to the DynamoDB API by result",
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
)

func init() {
	_ = metrics.RegisterPrometheusCollectors(apiRequests, apiRequestsTotal, apiRequestLatencies)
}

// TableType indicates which type of table metrics are being calculated for
type TableType string

const (
	// Backend is a table used to store backend data.
	Backend TableType = "backend"
	// Events is a table used to store audit events.
	Events TableType = "events"
)

// MetricsMiddleware returns middleware that can be used to capture
// prometheus metrics for interacting with DynamoDB.
func MetricsMiddleware(tableType TableType) []func(stack *middleware.Stack) error {
	type timestampKey struct{}

	return []func(s *middleware.Stack) error{
		func(stack *middleware.Stack) error {
			return stack.Initialize.Add(middleware.InitializeMiddlewareFunc(
				"DynamoMetricsBefore",
				func(ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler) (middleware.InitializeOutput, middleware.Metadata, error) {
					return next.HandleInitialize(context.WithValue(ctx, timestampKey{}, time.Now()), in)
				}), middleware.Before)
		},
		func(stack *middleware.Stack) error {
			return stack.Initialize.Add(middleware.InitializeMiddlewareFunc(
				"DynamoMetricsAfter",
				func(ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler) (middleware.InitializeOutput, middleware.Metadata, error) {
					out, md, err := next.HandleInitialize(ctx, in)

					result := "success"
					if err != nil {
						result = "error"
					}

					then := ctx.Value(timestampKey{}).(time.Time)
					operation := awsmiddleware.GetOperationName(ctx)
					latency := time.Since(then).Seconds()

					apiRequestsTotal.WithLabelValues(string(tableType), operation).Inc()
					apiRequestLatencies.WithLabelValues(string(tableType), operation).Observe(latency)
					apiRequests.WithLabelValues(string(tableType), operation, result).Inc()

					return out, md, err
				}), middleware.After)
		},
	}
}
