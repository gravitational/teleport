// Copyright 2022 Gravitational, Inc
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

package breaker

import (
	"context"

	"google.golang.org/grpc"
)

// UnaryClientInterceptor is a unary gRPC client interceptor that uses the provided CircuitBreaker to track errors
// returned from the outgoing calls.
func UnaryClientInterceptor(cb *CircuitBreaker) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		_, err := cb.Execute(func() (interface{}, error) {
			err := invoker(ctx, method, req, reply, cc, opts...)
			return nil, err
		})
		return err
	}
}

// StreamClientInterceptor is a stream gRPC client interceptor that uses the provided CircuitBreaker to track errors
// returned from the outgoing calls.
func StreamClientInterceptor(cb *CircuitBreaker) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		stream, err := cb.Execute(func() (interface{}, error) {
			return streamer(ctx, desc, cc, method, opts...)
		})

		if stream == nil {
			return nil, err
		}

		if cs, ok := stream.(grpc.ClientStream); ok {
			return cs, err
		}

		return nil, err
	}
}
