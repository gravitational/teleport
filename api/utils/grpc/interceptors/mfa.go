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

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/mfa"
)

// RetryWithMFAUnaryInterceptor intercepts a GRPC client unary call to check if the
// error indicates that the client should retry with MFA verification.
func RetryWithMFAUnaryInterceptor(mfaCeremony func(ctx context.Context) (*proto.MFAAuthenticateResponse, error)) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		err := invoker(ctx, method, req, reply, cc, opts...)
		if !errors.Is(trail.FromGRPC(err), &mfa.ErrAdminActionMFARequired) {
			return err
		}

		mfaResp, ceremonyErr := mfaCeremony(ctx)
		if ceremonyErr != nil {
			return trace.NewAggregate(trail.FromGRPC(err), ceremonyErr)
		}

		return invoker(ctx, method, req, reply, cc, append(opts, mfa.WithCredentials(mfaResp))...)
	}
}
