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

package clusters

import (
	"context"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/auth"
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
func (c *Cluster) GetServers(ctx context.Context, r *api.GetServersRequest) (*GetServersResponse, error) {
	var (
		page        apiclient.ResourcePage[types.Server]
		authClient  auth.ClientI
		proxyClient *client.ProxyClient
		err         error
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

	err = addMetadataToRetryableError(ctx, func() error {
		proxyClient, err = c.clusterClient.ConnectToProxy(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()

		authClient, err = proxyClient.ConnectToCluster(ctx, c.clusterClient.SiteName)
		if err != nil {
			return trace.Wrap(err)
		}
		defer authClient.Close()

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
