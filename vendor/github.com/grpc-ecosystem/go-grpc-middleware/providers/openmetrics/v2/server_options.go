// Copyright (c) The go-grpc-middleware Authors.
// Licensed under the Apache License 2.0.

package metrics

import (
	openmetrics "github.com/prometheus/client_golang/prometheus"
)

type serverMetricsConfig struct {
	counterOpts counterOptions
	// serverHandledHistogram can be nil
	serverHandledHistogram *openmetrics.HistogramVec
}

type ServerMetricsOption func(*serverMetricsConfig)

func (c *serverMetricsConfig) apply(opts []ServerMetricsOption) {
	for _, o := range opts {
		o(c)
	}
}

func WithServerCounterOptions(opts ...CounterOption) ServerMetricsOption {
	return func(o *serverMetricsConfig) {
		o.counterOpts = opts
	}
}

// WithServerHandlingTimeHistogram turns on recording of handling time of RPCs.
// Histogram metrics can be very expensive for Prometheus to retain and query.
func WithServerHandlingTimeHistogram(opts ...HistogramOption) ServerMetricsOption {
	return func(o *serverMetricsConfig) {
		o.serverHandledHistogram = openmetrics.NewHistogramVec(
			histogramOptions(opts).apply(openmetrics.HistogramOpts{
				Name:    "grpc_server_handling_seconds",
				Help:    "Histogram of response latency (seconds) of gRPC that had been application-level handled by the server.",
				Buckets: openmetrics.DefBuckets,
			}),
			[]string{"grpc_type", "grpc_service", "grpc_method"},
		)
	}
}
