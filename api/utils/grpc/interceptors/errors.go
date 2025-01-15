// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package interceptors

import (
	"context"
	"errors"
	"io"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/trail"
)

// grpcServerStreamWrapper wraps around the embedded grpc.ServerStream
// and intercepts the RecvMsg and SendMsg method calls converting errors
// to the appropriate gRPC status error.
type grpcServerStreamWrapper struct {
	grpc.ServerStream
}

// SendMsg wraps around ServerStream.SendMsg and adds metrics reporting
func (s *grpcServerStreamWrapper) SendMsg(m interface{}) error {
	return trace.Unwrap(trail.FromGRPC(s.ServerStream.SendMsg(m)))
}

// RecvMsg wraps around ServerStream.RecvMsg and adds metrics reporting
func (s *grpcServerStreamWrapper) RecvMsg(m interface{}) error {
	return trace.Unwrap(trail.FromGRPC(s.ServerStream.RecvMsg(m)))
}

// grpcClientStreamWrapper wraps around the embedded grpc.ClientStream
// and intercepts the RecvMsg and SendMsg method calls converting errors
// to the appropriate gRPC status error.
type grpcClientStreamWrapper struct {
	grpc.ClientStream
}

// SendMsg wraps around ClientStream.SendMsg
func (s *grpcClientStreamWrapper) SendMsg(m interface{}) error {
	return wrapStreamErr(s.ClientStream.SendMsg(m))
}

// RecvMsg wraps around ClientStream.RecvMsg
func (s *grpcClientStreamWrapper) RecvMsg(m interface{}) error {
	return wrapStreamErr(s.ClientStream.RecvMsg(m))
}

func wrapStreamErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, io.EOF):
		// Do not wrap io.EOF errors, they are often used as stop guards for streams.
		return err
	default:
		return &RemoteError{Err: trace.Unwrap(trail.FromGRPC(err))}
	}
}

// GRPCServerUnaryErrorInterceptor is a gRPC unary server interceptor that
// handles converting errors to the appropriate gRPC status error.
func GRPCServerUnaryErrorInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	resp, err := handler(ctx, req)
	return resp, trace.Unwrap(trail.ToGRPC(err))
}

// RemoteError annotates server-side errors translated into trace by
// [GRPCClientUnaryErrorInterceptor] or [GRPCClientStreamErrorInterceptor].
type RemoteError struct {
	// Err is the underlying error.
	Err error
}

func (e *RemoteError) Error() string {
	if e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *RemoteError) Unwrap() error {
	return e.Err
}

// GRPCClientUnaryErrorInterceptor is a gRPC unary client interceptor that
// handles converting errors to the appropriate grpc status error.
func GRPCClientUnaryErrorInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	if err := invoker(ctx, method, req, reply, cc, opts...); err != nil {
		return &RemoteError{Err: trace.Unwrap(trail.FromGRPC(err))}
	}
	return nil
}

// GRPCServerStreamErrorInterceptor is a gRPC server stream interceptor that
// handles converting errors to the appropriate gRPC status error.
func GRPCServerStreamErrorInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	serverWrapper := &grpcServerStreamWrapper{ss}
	return trace.Unwrap(trail.ToGRPC(handler(srv, serverWrapper)))
}

// GRPCClientStreamErrorInterceptor is gRPC client stream interceptor that
// handles converting errors to the appropriate gRPC status error.
func GRPCClientStreamErrorInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	s, err := streamer(ctx, desc, cc, method, opts...)
	if err != nil {
		return nil, &RemoteError{Err: trace.Unwrap(trail.ToGRPC(err))}
	}
	return &grpcClientStreamWrapper{s}, nil
}
