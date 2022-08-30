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
	"strings"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	api "github.com/gravitational/teleport/lib/teleterm/api/protogen/golang/v1"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"

	"github.com/gravitational/trace"
)

// Database describes database
type Server struct {
	// URI is the database URI
	URI uri.ResourceURI

	types.Server
}

// GetAllServers returns a full list of nodes without pagination or sorting.
func (c *Cluster) GetAllServers(ctx context.Context) ([]Server, error) {
	var clusterServers []types.Server
	err := addMetadataToRetryableError(ctx, func() error {
		proxyClient, err := c.clusterClient.ConnectToProxy(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()

		clusterServers, err = proxyClient.FindNodesByFilters(ctx, proto.ListResourcesRequest{
			Namespace: defaults.Namespace,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	results := []Server{}
	for _, server := range clusterServers {
		results = append(results, Server{
			URI:    c.URI.AppendServer(server.GetName()),
			Server: server,
		})
	}

	return results, nil
}

func (c *Cluster) GetServers(ctx context.Context, r *api.GetServersRequest) (*GetServersResponse, error) {
	var (
		clusterServers []types.Server
		resp           *types.ListResourcesResponse
		sortBy         types.SortBy
	)

	err := addMetadataToRetryableError(ctx, func() error {
		proxyClient, err := c.clusterClient.ConnectToProxy(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()

		authClient, err := proxyClient.CurrentClusterAccessPoint(ctx)
		// do we need to call authClient.Close() similar to proxyClient above?
		// is there a better way to get the auth client instead of this?
		if err != nil {
			return trace.Wrap(err)
		}

		sortParam := r.SortBy
		if sortParam != "" {
			vals := strings.Split(sortParam, ":")
			if vals[0] != "" {
				sortBy.Field = vals[0]
				if len(vals) > 1 && vals[1] == "desc" {
					sortBy.IsDesc = true
				}
			}
		}

		resp, err = authClient.ListResources(ctx, proto.ListResourcesRequest{
			Namespace:           defaults.Namespace,
			ResourceType:        types.KindNode,
			Limit:               r.Limit,
			SortBy:              sortBy,
			PredicateExpression: r.Query,
			SearchKeywords:      client.ParseSearchKeywords(r.Search, ' '),
			UseSearchAsRoles:    r.SearchAsRoles == "yes",
		})
		clusterServers, err = types.ResourcesWithLabels(resp.Resources).AsServers()

		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	results := []Server{}
	for _, server := range clusterServers {
		results = append(results, Server{
			URI:    c.URI.AppendServer(server.GetName()),
			Server: server,
		})
	}

	return &GetServersResponse{
		Servers:    results,
		StartKey:   resp.NextKey,
		TotalCount: resp.TotalCount,
	}, nil
}

type GetServersResponse struct {
	// Resources is a list of resource.
	Servers []Server
	// NextKey is the next key to use as a starting point.
	StartKey string
	// // TotalCount is the total number of resources available as a whole.
	TotalCount int
}
