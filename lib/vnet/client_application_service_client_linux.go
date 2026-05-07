// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package vnet

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

// newUnixClientApplicationServiceClient creates a gRPC client over a Unix
// socket without TLS.
func newUnixClientApplicationServiceClient(_ context.Context, socketPath string) (*clientApplicationServiceClient, error) {
	if socketPath == "" {
		return nil, trace.BadParameter("missing unix socket path")
	}
	conn, err := grpc.NewClient(fmt.Sprintf("unix://%s", socketPath),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(interceptors.GRPCClientUnaryErrorInterceptor),
		grpc.WithStreamInterceptor(interceptors.GRPCClientStreamErrorInterceptor),
	)
	if err != nil {
		return nil, trace.Wrap(err, "creating user process gRPC client")
	}
	return &clientApplicationServiceClient{
		clt:    vnetv1.NewClientApplicationServiceClient(conn),
		closer: conn,
	}, nil
}
