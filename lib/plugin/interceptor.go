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

package plugin

import (
	"context"

	"github.com/gravitational/teleport/lib/utils"

	"google.golang.org/grpc"
)

// Interceptor defines configurable grpc server interceptors.
type Interceptor interface {
	UnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (response interface{}, err error)
	StreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error

	// RegisterUnaryInterceptor registers a UnaryServerInterceptor
	RegisterUnaryInterceptor(unaryInterceptor grpc.UnaryServerInterceptor)
	// RegisterStreamInterceptor registers a StreamServerInterceptor
	RegisterStreamInterceptor(streamInterceptor grpc.StreamServerInterceptor)
}

type interceptor struct {
	unaryInterceptor  grpc.UnaryServerInterceptor
	streamInterceptor grpc.StreamServerInterceptor
}

// NewInterceptor creates a new Interceptor.
func NewInterceptor() Interceptor {
	return &interceptor{}
}

// UnaryInterceptor invokes the registered unary interceptor or does nothing if a
// unary interceptor has not been registered.
func (r *interceptor) UnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (response interface{}, err error) {
	if r.unaryInterceptor != nil {
		return r.unaryInterceptor(ctx, req, info, handler)
	}
	return handler(ctx, req)
}

// StreamInterceptor invokes the registered stream interceptor or does nothing if
// a stream interceptor has not been registered.
func (r *interceptor) StreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if r.streamInterceptor != nil {
		return r.streamInterceptor(srv, ss, info, handler)
	}
	return handler(srv, ss)
}

// RegisterUnaryInterceptor registers a unary interceptor.
func (r *interceptor) RegisterUnaryInterceptor(unaryInterceptor grpc.UnaryServerInterceptor) {
	if r.unaryInterceptor == nil {
		r.unaryInterceptor = unaryInterceptor
		return
	}
	r.unaryInterceptor = utils.ChainUnaryServerInterceptors(r.unaryInterceptor, unaryInterceptor)
}

// RegisterStreamInterceptor registers a stream interceptor.
func (r *interceptor) RegisterStreamInterceptor(streamInterceptor grpc.StreamServerInterceptor) {
	if r.streamInterceptor == nil {
		r.streamInterceptor = streamInterceptor
		return
	}
	r.streamInterceptor = utils.ChainStreamServerInterceptors(r.streamInterceptor, streamInterceptor)
}
