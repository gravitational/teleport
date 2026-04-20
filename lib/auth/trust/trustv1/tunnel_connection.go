/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

	trustpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	"github.com/gravitational/teleport/api/types"
)

// ListTunnelConnections returns a page of tunnel connection resources,
// optionally filtered by cluster name.
func (s *Service) ListTunnelConnections(ctx context.Context, req *trustpb.ListTunnelConnectionsRequest) (*trustpb.ListTunnelConnectionsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindTunnelConnection, types.VerbList, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	conns, next, err := s.cache.ListTunnelConnections(ctx, int(req.PageSize), req.PageToken, req.Filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp := &trustpb.ListTunnelConnectionsResponse{
		TunnelConnections: make([]*types.TunnelConnectionV2, 0, len(conns)),
		NextPageToken:     next,
	}
	for _, c := range conns {
		v2, ok := c.(*types.TunnelConnectionV2)
		if !ok {
			return nil, trace.Errorf("unexpected TunnelConnection type: %T", c)
		}
		resp.TunnelConnections = append(resp.TunnelConnections, v2)
	}
	return resp, nil
}
