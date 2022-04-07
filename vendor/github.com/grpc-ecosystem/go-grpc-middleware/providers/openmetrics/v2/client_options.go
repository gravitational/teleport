// Copyright (c) The go-grpc-middleware Authors.
// Licensed under the Apache License 2.0.

package metrics

import (
	openmetrics "github.com/prometheus/client_golang/prometheus"
)

type clientMetricsConfig struct {
	counterOpts counterOptions
	// clientHandledHistogram can be nil
	clientHandledHistogram *openmetrics.HistogramVec
	// clientStreamRecvHistogram can be nil
	clientStreamRecvHistogram *openmetrics.HistogramVec
	// clientStreamSendHistogram can be nil
	clientStreamSendHistogram *openmetrics.HistogramVec
}

type ClientMetricsOption func(*clientMetricsConfig)

func (c *clientMetricsConfig) apply(opts []ClientMetricsOption) {
	for _, o := range opts {
		o(c)
	}
}

func WithClientCounterOptions(opts ...CounterOption) ClientMetricsOption {
	return func(o *clientMetricsConfig) {
		o.counterOpts = opts
	}
}

// WithClientHandlingTimeHistogram turns on recording of handling time of RPCs.
// Histogram metrics can be very expensive for Prometheus to retain and query.
func WithClientHandlingTimeHistogram(opts ...HistogramOption) ClientMetricsOption {
	return func(o *clientMetricsConfig) {
		o.clientHandledHistogram = openmetrics.NewHistogramVec(
			histogramOptions(opts).apply(openmetrics.HistogramOpts{
				Name:    "grpc_client_handling_seconds",
				Help:    "Histogram of response latency (seconds) of the gRPC until it is finished by the application.",
				Buckets: openmetrics.DefBuckets,
			}),
			[]string{"grpc_type", "grpc_service", "grpc_method"},
		)
	}
}

// WithClientStreamRecvHistogram turns on recording of single message receive time of streaming RPCs.
// Histogram metrics can be very expensive for Prometheus to retain and query.
func WithClientStreamRecvHistogram(opts ...HistogramOption) ClientMetricsOption {
	return func(o *clientMetricsConfig) {
		o.clientStreamRecvHistogram = openmetrics.NewHistogramVec(
			histogramOptions(opts).apply(openmetrics.HistogramOpts{
				Name:    "grpc_client_msg_recv_handling_seconds",
				Help:    "Histogram of response latency (seconds) of the gRPC single message receive.",
				Buckets: openmetrics.DefBuckets,
			}),
			[]string{"grpc_type", "grpc_service", "grpc_method"},
		)
	}
}

// WithClientStreamSendHistogram turns on recording of single message send time of streaming RPCs.
// Histogram metrics can be very expensive for Prometheus to retain and query.
func WithClientStreamSendHistogram(opts ...HistogramOption) ClientMetricsOption {
	return func(o *clientMetricsConfig) {
		o.clientStreamSendHistogram = openmetrics.NewHistogramVec(
			histogramOptions(opts).apply(openmetrics.HistogramOpts{
				Name:    "grpc_client_msg_send_handling_seconds",
				Help:    "Histogram of response latency (seconds) of the gRPC single message send.",
				Buckets: openmetrics.DefBuckets,
			}),
			[]string{"grpc_type", "grpc_service", "grpc_method"},
		)
	}
}
