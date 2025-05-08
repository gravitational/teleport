/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package clusters

import (
	"context"

	"github.com/gravitational/teleport/api/types"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/trace"
)

// ListDatabaseServers returns a paginated list of database servers (resource kind "db_server").
func (c *Cluster) ListDatabaseServers(ctx context.Context, r *api.ListResourcesRequest, authClient authclient.ClientI) (*api.ListDatabaseServersResponse, error) {
	page, err := ListResources[types.DatabaseServer](ctx, r, authClient, types.KindDatabaseServer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := &api.ListDatabaseServersResponse{
		NextKey: page.NextKey,
	}

	for _, resource := range page.Resources {
		dbServerResource, ok := resource.(*types.DatabaseServerV3)
		if !ok {
			return nil, trace.BadParameter("expected resource type %T, got %T", types.DatabaseServerV3{}, resource)
		}
		response.Servers = append(response.Servers, &api.DatabaseServer{
			ClusterUri: c.URI.AppendDBServer(dbServerResource.GetName()).String(),
			Hostname:   dbServerResource.GetHostname(),
			HostId:     dbServerResource.GetHostID(),
			TargetHealth: &api.TargetHealth{
				Status: dbServerResource.GetTargetHealth().Status,
				Error:  dbServerResource.GetTargetHealth().TransitionError,
			},
		})
	}

	return response, nil
}
