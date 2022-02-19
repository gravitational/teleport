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

	"github.com/gravitational/trace/trail"
	"google.golang.org/grpc"
)

func ErrorConvertUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	resp, err := handler(ctx, req)
	return resp, trail.ToGRPC(err)
}

func ErrorConvertStreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return trail.ToGRPC(handler(srv, ss))
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
