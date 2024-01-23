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
	"strings"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/mfa"
)

// WithMFAUnaryInterceptor intercepts a GRPC client unary call to add MFA credentials
// to the rpc call when an MFA response is provided through the context. Additionally,
// when the call returns an error that indicates that MFA is required, this interceptor
// will prompt for MFA using the given mfaCeremony and retry.
func WithMFAUnaryInterceptor(clt mfa.MFACeremonyClient) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// Check for MFA response passed through the context.
		if mfaResp, err := mfa.MFAResponseFromContext(ctx); err == nil {
			return invoker(ctx, method, req, reply, cc, append(opts, mfa.WithCredentials(mfaResp))...)
		} else if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}

		err := invoker(ctx, method, req, reply, cc, opts...)
		if !errors.Is(trail.FromGRPC(err), &mfa.ErrAdminActionMFARequired) {
			return err
		}

		// In this context, method looks like "/proto.<grpc-service-name>/<method-name>",
		// we just want the method name.
		splitMethod := strings.Split(method, "/")
		readableMethodName := splitMethod[len(splitMethod)-1]

		// Start an MFA prompt that shares what API request caused MFA to be prompted.
		// ex: MFA is required for admin-level API request: "CreateUser"
		mfaResp, ceremonyErr := mfa.PerformAdminActionMFACeremony(ctx, clt, readableMethodName, false /*allowReuse*/)
		if ceremonyErr != nil {
			return trace.NewAggregate(trail.FromGRPC(err), ceremonyErr)
		}

		return invoker(ctx, method, req, reply, cc, append(opts, mfa.WithCredentials(mfaResp))...)
	}
}
