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

package apiserver

import (
	"context"
	"log/slog"
	"runtime/debug"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport/api/trail"
)

// withUnaryErrorHandling is gRPC middleware that maps internal errors from unary
// handlers to proper gRPC error codes.
func withUnaryErrorHandling(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			log.ErrorContext(ctx, "Request failed", "error", err)
			return resp, trail.ToGRPC(err)
		}

		return resp, nil
	}
}

// withStreamErrorHandling is gRPC middleware that maps internal errors from streaming
// handlers to proper gRPC error codes.
func withStreamErrorHandling(log *slog.Logger) grpc.StreamServerInterceptor {
	return func(
		srv any,
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		err := handler(srv, stream)
		if err != nil {
			log.ErrorContext(stream.Context(), "Stream request failed", "error", err)
			return trail.ToGRPC(err)
		}

		return nil
	}
}

// withUnaryPanicRecovery is gRPC middleware that recovers from panics in unary handlers.
func withUnaryPanicRecovery(log *slog.Logger) grpc.UnaryServerInterceptor {
	return recovery.UnaryServerInterceptor(panicRecoveryOption(log))
}

// withStreamPanicRecovery is gRPC middleware that recovers from panics in streaming handlers.
func withStreamPanicRecovery(log *slog.Logger) grpc.StreamServerInterceptor {
	return recovery.StreamServerInterceptor(panicRecoveryOption(log))
}

func panicRecoveryOption(log *slog.Logger) recovery.Option {
	return recovery.WithRecoveryHandlerContext(func(ctx context.Context, p any) error {
		log.ErrorContext(ctx, "Recovered from panic in gRPC handler",
			"panic", p,
			"stack", string(debug.Stack()),
		)
		return status.Errorf(codes.Internal, "handler panic: %v", p)
	})
}
