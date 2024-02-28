/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package apiserver

import (
	"context"
	"errors"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client"
)

// withErrorHandling is gRPC middleware that maps internal errors to proper gRPC error codes
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
			// A stop gap solution that allows us to show a relogin modal when we
			// receive an error from the server saying that the cert is expired.
			// Read more: https://github.com/gravitational/teleport/pull/38202#discussion_r1497181659
			// TODO(gzdunek): fix when addressing https://github.com/gravitational/teleport/issues/32550
			if errors.Is(err, client.ErrClientCredentialsHaveExpired) {
				return resp, trail.ToGRPC(err)
			}

			// do not return a full error stack on access denied errors
			if trace.IsAccessDenied(err) {
				return resp, trail.ToGRPC(trace.AccessDenied("access denied"))
			}
			return resp, trail.ToGRPC(err)
		}

		return resp, nil
	}
}
