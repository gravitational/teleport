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

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/lib/defaults"
)

func (e *Engine) newGRPCServer() (*grpc.Server, error) {
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(e.unaryServerInterceptors()...),
		grpc.ChainStreamInterceptor(e.streamServerInterceptors()...),
		grpc.MaxConcurrentStreams(defaults.GRPCMaxConcurrentStreams),
	)
	return server, nil
}

func (e *Engine) unaryServerInterceptors() []grpc.UnaryServerInterceptor {
	// intercept and log some info, then convert errors to gRPC codes.
	return []grpc.UnaryServerInterceptor{
		interceptors.GRPCServerUnaryErrorInterceptor,
		unaryServerLoggingInterceptor(e.Log),
	}
}

func (e *Engine) streamServerInterceptors() []grpc.StreamServerInterceptor {
	// intercept and log some info, then convert errors to gRPC codes.
	return []grpc.StreamServerInterceptor{
		interceptors.GRPCServerStreamErrorInterceptor,
		streamServerLoggingInterceptor(e.Log),
	}
}

// unaryServerLoggingInterceptor is gRPC middleware that logs some debug info.
func unaryServerLoggingInterceptor(log logrus.FieldLogger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		res, err := handler(ctx, req)
		log.WithError(err).WithField("full_method", info.FullMethod).Debug("Handled unary gRPC request")
		return res, err
	}
}

// streamServerLoggingInterceptor is gRPC middleware that logs some debug info.
func streamServerLoggingInterceptor(log logrus.FieldLogger) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		err := handler(srv, stream)
		log.WithError(err).WithField("full_method", info.FullMethod).Debug("Handled streaming gRPC request")
		return err
	}
}
