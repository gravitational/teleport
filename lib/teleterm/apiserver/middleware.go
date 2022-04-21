/*
Copyright 2021 Gravitational, Inc.

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

package apiserver

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// withErrorHandling is GRPC middleware that maps internal errors to proper GRPC error codes
func withErrorHandling(log logrus.FieldLogger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			log.WithError(err).Error("Request failed.")
			// do not return a full error stack on access denied errors
			if trace.IsAccessDenied(err) {
				return resp, trail.ToGRPC(trace.AccessDenied("access denied"))
			}
			return resp, trail.ToGRPC(err)
		}

		return resp, nil
	}
}
