/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package spanner

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
)

// unaryServerLoggingInterceptor is gRPC middleware that logs some debug info.
func unaryServerLoggingInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		res, err := handler(ctx, req)
		logRPC(log, info.FullMethod, err)
		return res, err
	}
}

// streamServerLoggingInterceptor is gRPC middleware that logs some debug info.
func streamServerLoggingInterceptor(log *slog.Logger) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		err := handler(srv, stream)
		logRPC(log, info.FullMethod, err)
		return err
	}
}

func logRPC(log *slog.Logger, fullMethod string, handlerErr error) {
	if handlerErr != nil {
		log.Debug("failed to handle Spanner RPC", "full_method", fullMethod, "error", handlerErr)
		return
	}
}
