/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"google.golang.org/grpc"
)

// grpcServerStreamWrapper wraps around the embedded grpc.ServerStream
// and intercepts the RecvMsg and SendMsg method calls converting errors to the
// appropriate grpc status error.
type grpcServerStreamWrapper struct {
	grpc.ServerStream
}

// SendMsg wraps around ServerStream.SendMsg and adds metrics reporting
func (s *grpcServerStreamWrapper) SendMsg(m interface{}) error {
	return trail.FromGRPC(s.ServerStream.SendMsg(m))
}

// RecvMsg wraps around ServerStream.RecvMsg and adds metrics reporting
func (s *grpcServerStreamWrapper) RecvMsg(m interface{}) error {
	return trail.FromGRPC(s.ServerStream.RecvMsg(m))
}

// grpcClientStreamWrapper wraps around the embedded grpc.ClientStream
// and intercepts the RecvMsg and SendMsg method calls converting errors to the
// appropriate grpc status error.
type grpcClientStreamWrapper struct {
	grpc.ClientStream
}

// SendMsg wraps around ClientStream.SendMsg
func (s *grpcClientStreamWrapper) SendMsg(m interface{}) error {
	return trail.FromGRPC(s.ClientStream.SendMsg(m))
}

// RecvMsg wraps around ClientStream.RecvMsg
func (s *grpcClientStreamWrapper) RecvMsg(m interface{}) error {
	return trail.FromGRPC(s.ClientStream.RecvMsg(m))
}

// GRPCServerUnaryErrorInterceptor is a GPRC unary server interceptor that
// handles converting errors to the appropriate grpc status error.
func GRPCServerUnaryErrorInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	resp, err := handler(ctx, req)
	return resp, trail.ToGRPC(err)
}

// GRPCClientUnaryErrorInterceptor is a GPRC unary client interceptor that
// handles converting errors to the appropriate grpc status error.
func GRPCClientUnaryErrorInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	return trail.FromGRPC(invoker(ctx, method, req, reply, cc, opts...))
}

// GRPCServerStreamErrorInterceptor is a GPRC server stream interceptor that
// handles converting errors to the appropriate grpc status error.
func GRPCServerStreamErrorInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	serverWrapper := &grpcServerStreamWrapper{ss}
	return trail.ToGRPC(handler(srv, serverWrapper))
}

// GRPCClientStreamErrorInterceptor is GPRC client stream interceptor that
// handles converting errors to the appropriate grpc status error.
func GRPCClientStreamErrorInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	s, err := streamer(ctx, desc, cc, method, opts...)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &grpcClientStreamWrapper{s}, nil
}

// ChainUnaryServerInterceptors takes 1 or more grpc.UnaryServerInterceptors and
// chains them into a single grpc.UnaryServerInterceptor. The first interceptor
// will be the outer most, while the last interceptor will be the inner most
// wrapper around the real call.
func ChainUnaryServerInterceptors(first grpc.UnaryServerInterceptor, rest ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		for i := len(rest) - 1; i >= 0; i-- {
			// wrap the current handler with the current interceptor
			// create local variables scoped to the current loop iteration so
			// they will be correctly captured.
			currentHandler := handler
			currentInterceptor := rest[i]
			handler = func(ctx context.Context, req interface{}) (interface{}, error) {
				return currentInterceptor(ctx, req, info, currentHandler)
			}
		}
		// call the first interceptor with the wrapped handler
		return first(ctx, req, info, handler)
	}
}

// ChainStreamServerInterceptors takes 1 or more grpc.StreamServerInterceptors and
// chains them into a single grpc.StreamServerInterceptor. The first interceptor
// will be the outer most, while the last interceptor will be the inner most
// wrapper around the real call.
func ChainStreamServerInterceptors(first grpc.StreamServerInterceptor, rest ...grpc.StreamServerInterceptor) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		for i := len(rest) - 1; i >= 0; i-- {
			// wrap the current handler with the current interceptor
			// create local variables scoped to the current loop iteration so
			// they will be correctly captured.
			currentHandler := handler
			currentInterceptor := rest[i]
			handler = func(srv interface{}, stream grpc.ServerStream) error {
				return currentInterceptor(srv, ss, info, currentHandler)
			}
		}
		// call the first interceptor with the wrapped handler
		return first(srv, ss, info, handler)
	}
}

// NewGRPCDummyClientConnection returns an implementation of grpc.ClientConnInterface
// that always responds with "not implemented" error with the given error message.
func NewGRPCDummyClientConnection(message string) grpc.ClientConnInterface {
	return &grpcDummyClientConnection{
		message: message,
	}
}

type grpcDummyClientConnection struct {
	message string
}

// Invoke implements grpc.ClientConnInterface
func (g *grpcDummyClientConnection) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	return trace.NotImplemented("%s: %s", method, g.message)
}

// NewStream implements grpc.ClientConnInterface
func (g *grpcDummyClientConnection) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, trace.NotImplemented("%s: %s", method, g.message)
}
