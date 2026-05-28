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

package handler

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

// ListRequestableRoles returns a list of roles that the user can request.
func (h *Handler) ListRequestableRoles(ctx context.Context, req *api.ListRequestableRolesRequest) (*api.ListRequestableRolesResponse, error) {
	rootClusterURI, err := uri.Parse(req.GetRootClusterUri())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyClient, err := h.DaemonService.GetCachedClient(ctx, rootClusterURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authRes, err := proxyClient.AuthClient.ListRequestableRoles(ctx, &proto.ListRequestableRolesRequest{
		PageSize:  req.PageSize,
		PageToken: req.PageToken,
		Filter:    req.Filter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res := &api.ListRequestableRolesResponse{
		NextPageToken: authRes.NextPageToken,
		Roles:         make([]*api.Role, 0, len(authRes.Roles)),
	}

	for _, role := range authRes.Roles {
		res.Roles = append(res.Roles, api.Role_builder{
			Name:        role.GetName(),
			Description: role.GetDescription(),
		}.Build())
	}

	return res, nil
}
