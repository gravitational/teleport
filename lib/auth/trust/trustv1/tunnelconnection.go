/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

package trustv1

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	trustpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	"github.com/gravitational/teleport/api/types"
)

// UpsertTunnelConnection creates or updates the provided tunnel connection.
func (s *Service) UpsertTunnelConnection(ctx context.Context, req *trustpb.UpsertTunnelConnectionRequest) (*trustpb.UpsertTunnelConnectionResponse, error) {
	if req.TunnelConnection == nil {
		return nil, trace.BadParameter("missing tunnel connection")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindTunnelConnection, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.UpsertTunnelConnection(ctx, req.TunnelConnection); err != nil {
		return nil, trace.Wrap(err)
	}

	return &trustpb.UpsertTunnelConnectionResponse{
		TunnelConnection: req.TunnelConnection,
	}, nil
}

// DeleteTunnelConnection removes a single tunnel connection by cluster and
// connection name.
func (s *Service) DeleteTunnelConnection(ctx context.Context, req *trustpb.DeleteTunnelConnectionRequest) (*emptypb.Empty, error) {
	if req.ClusterName == "" {
		return nil, trace.BadParameter("missing cluster name")
	}
	if req.ConnectionName == "" {
		return nil, trace.BadParameter("missing connection name")
	}

	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindTunnelConnection, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.backend.DeleteTunnelConnection(ctx, req.ClusterName, req.ConnectionName); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}
