/*
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

package aws

import (
	"context"
	"time"

	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/smithy-go/middleware"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/observability/metrics"
)

var (
	apiRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "aws_sdk_requests_total",
			Help:      "Total number of requests to the AWS API",
		},
		[]string{"service", "operation"},
	)
	apiRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "aws_sdk_requests",
			Help:      "Number of requests to the AWS API by result",
		},
		[]string{"service", "operation", "result"},
	)
	apiRequestLatencies = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "aws_sdk_requests_seconds",
			Help:      "Request latency for the AWS API",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
		[]string{"service", "operation"},
	)
)

func init() {
	_ = metrics.RegisterPrometheusCollectors(apiRequests, apiRequestsTotal, apiRequestLatencies)
}

// MetricsMiddleware returns middleware that can be used to capture
// prometheus metrics for interacting with the AWS API.
func MetricsMiddleware() []func(stack *middleware.Stack) error {
	type timestampKey struct{}

	return []func(s *middleware.Stack) error{
		func(stack *middleware.Stack) error {
			return stack.Initialize.Add(middleware.InitializeMiddlewareFunc(
				"AWSMetricsBefore",
				func(ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler) (middleware.InitializeOutput, middleware.Metadata, error) {
					return next.HandleInitialize(context.WithValue(ctx, timestampKey{}, time.Now()), in)
				}), middleware.Before)
		},
		func(stack *middleware.Stack) error {
			return stack.Initialize.Add(middleware.InitializeMiddlewareFunc(
				"AWSMetricsAfter",
				func(ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler) (middleware.InitializeOutput, middleware.Metadata, error) {
					out, md, err := next.HandleInitialize(ctx, in)

					result := "success"
					if err != nil {
						result = "error"
					}

					then := ctx.Value(timestampKey{}).(time.Time)
					service := awsmiddleware.GetServiceID(ctx)
					operation := awsmiddleware.GetOperationName(ctx)
					latency := time.Since(then).Seconds()

					apiRequestsTotal.WithLabelValues(service, operation).Inc()
					apiRequestLatencies.WithLabelValues(service, operation).Observe(latency)
					apiRequests.WithLabelValues(service, operation, result).Inc()

					return out, md, err
				}), middleware.After)
		},
	}
}
