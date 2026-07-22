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
	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

// CreateGRPCServerMetrics creates server gRPC metrics configuration that is to be registered and used by the caller
// in an openmetrics unary and/or stream interceptor
func CreateGRPCServerMetrics(
	latencyEnabled bool, labels prometheus.Labels,
) *grpcprom.ServerMetrics {
	serverOpts := []grpcprom.ServerMetricsOption{
		grpcprom.WithServerCounterOptions(grpcprom.WithConstLabels(labels)),
	}
	if latencyEnabled {
		histOpts := grpcHistogramOpts(labels)
		serverOpts = append(
			serverOpts, grpcprom.WithServerHandlingTimeHistogram(histOpts...),
		)
	}
	return grpcprom.NewServerMetrics(serverOpts...)
}

// CreateGRPCClientMetrics creates client gRPC metrics configuration that is to be registered and used by the caller
// in an openmetrics unary and/or stream interceptor
func CreateGRPCClientMetrics(
	latencyEnabled bool, labels prometheus.Labels,
) *grpcprom.ClientMetrics {
	clientOpts := []grpcprom.ClientMetricsOption{
		grpcprom.WithClientCounterOptions(grpcprom.WithConstLabels(labels)),
	}
	if latencyEnabled {
		histOpts := grpcHistogramOpts(labels)
		clientOpts = append(
			clientOpts, grpcprom.WithClientHandlingTimeHistogram(histOpts...),
		)
	}
	return grpcprom.NewClientMetrics(clientOpts...)
}

func grpcHistogramOpts(labels prometheus.Labels) []grpcprom.HistogramOption {
	return []grpcprom.HistogramOption{
		grpcprom.WithHistogramBuckets(prometheus.ExponentialBuckets(0.001, 2, 16)),
		grpcprom.WithHistogramConstLabels(labels),
	}
}
