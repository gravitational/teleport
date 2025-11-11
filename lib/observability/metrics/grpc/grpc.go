/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
