// Copyright (c) The go-grpc-middleware Authors.
// Licensed under the Apache License 2.0.

package metrics

import (
	"google.golang.org/grpc"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors"
)

// UnaryServerInterceptor is a gRPC server-side interceptor that provides Prometheus monitoring for Unary RPCs.
func UnaryServerInterceptor(serverMetrics *ServerMetrics) grpc.UnaryServerInterceptor {
	return interceptors.UnaryServerInterceptor(&reportable{
		serverMetrics: serverMetrics,
	})
}

// StreamServerInterceptor is a gRPC server-side interceptor that provides Prometheus monitoring for Streaming RPCs.
func StreamServerInterceptor(serverMetrics *ServerMetrics) grpc.StreamServerInterceptor {
	return interceptors.StreamServerInterceptor(&reportable{
		serverMetrics: serverMetrics,
	})
}
