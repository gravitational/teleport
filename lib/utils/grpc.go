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
	"google.golang.org/grpc"
)

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
