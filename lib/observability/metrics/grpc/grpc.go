package grpcmetrics

import (
	prometheus2 "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

// CreateGRPCServerMetrics creates server gRPC metrics configuration that is to be registered and used by the caller
// in an openmetrics unary and/or stream interceptor
func CreateGRPCServerMetrics(
	latencyEnabled bool, labels prometheus.Labels,
) *prometheus2.ServerMetrics {
	serverOpts := []prometheus2.ServerMetricsOption{
		prometheus2.WithServerCounterOptions(prometheus2.WithConstLabels(labels)),
	}
	if latencyEnabled {
		histOpts := grpcHistogramOpts(labels)
		serverOpts = append(
			serverOpts, prometheus2.WithServerHandlingTimeHistogram(histOpts...),
		)
	}
	return prometheus2.NewServerMetrics(serverOpts...)
}

// CreateGRPCClientMetrics creates client gRPC metrics configuration that is to be registered and used by the caller
// in an openmetrics unary and/or stream interceptor
func CreateGRPCClientMetrics(
	latencyEnabled bool, labels prometheus.Labels,
) *prometheus2.ClientMetrics {
	clientOpts := []prometheus2.ClientMetricsOption{
		prometheus2.WithClientCounterOptions(prometheus2.WithConstLabels(labels)),
	}
	if latencyEnabled {
		histOpts := grpcHistogramOpts(labels)
		clientOpts = append(
			clientOpts, prometheus2.WithClientHandlingTimeHistogram(histOpts...),
		)
	}
	return prometheus2.NewClientMetrics(clientOpts...)
}

func grpcHistogramOpts(labels prometheus.Labels) []prometheus2.HistogramOption {
	return []prometheus2.HistogramOption{
		prometheus2.WithHistogramBuckets(prometheus.ExponentialBuckets(0.001, 2, 16)),
		prometheus2.WithHistogramConstLabels(labels),
	}
}
