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

package clusters

import (
	"context"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

// Database describes database
type Server struct {
	// URI is the database URI
	URI uri.ResourceURI

	types.Server
}

// GetServers returns a paginated list of servers.
func (c *Cluster) GetServers(ctx context.Context, r *api.GetServersRequest, authClient authclient.ClientI) (*GetServersResponse, error) {
	var (
		page apiclient.ResourcePage[types.Server]
		err  error
	)

	req := &proto.ListResourcesRequest{
		Namespace:           defaults.Namespace,
		ResourceType:        types.KindNode,
		Limit:               r.Limit,
		SortBy:              types.GetSortByFromString(r.SortBy),
		StartKey:            r.StartKey,
		PredicateExpression: r.Query,
		SearchKeywords:      client.ParseSearchKeywords(r.Search, ' '),
		UseSearchAsRoles:    r.SearchAsRoles == "yes",
	}

	err = AddMetadataToRetryableError(ctx, func() error {
		page, err = apiclient.GetResourcePage[types.Server](ctx, authClient, req)
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	results := make([]Server, 0, len(page.Resources))
	for _, server := range page.Resources {
		results = append(results, Server{
			URI:    c.URI.AppendServer(server.GetName()),
			Server: server,
		})
	}

	return &GetServersResponse{
		Servers:    results,
		StartKey:   page.NextKey,
		TotalCount: page.Total,
	}, nil
}

type GetServersResponse struct {
	Servers []Server
	// StartKey is the next key to use as a starting point.
	StartKey string
	// TotalCount is the total number of resources available as a whole.
	TotalCount int
}
