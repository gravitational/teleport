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

package handler

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

func (s *Handler) ListUnifiedResources(ctx context.Context, req *api.ListUnifiedResourcesRequest) (*api.ListUnifiedResourcesResponse, error) {
	clusterURI, err := uri.Parse(req.GetClusterUri())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sortBy := types.SortBy{}
	if req.GetSortBy() != nil {
		sortBy.IsDesc = req.GetSortBy().IsDesc
		sortBy.Field = req.GetSortBy().Field
	}

	daemonResponse, err := s.DaemonService.ListUnifiedResources(ctx, clusterURI, &proto.ListUnifiedResourcesRequest{
		Kinds:               req.GetKinds(),
		Limit:               req.GetLimit(),
		StartKey:            req.GetStartKey(),
		PredicateExpression: req.GetQuery(),
		SearchKeywords:      client.ParseSearchKeywords(req.GetSearch(), ' '),
		SortBy:              sortBy,
		UseSearchAsRoles:    req.GetSearchAsRoles(),
		PinnedOnly:          req.GetPinnedOnly(),
		IncludeRequestable:  req.GetIncludeRequestable(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := api.ListUnifiedResourcesResponse{
		Resources: []*api.PaginatedResource{}, NextKey: daemonResponse.NextKey,
	}

	for _, resource := range daemonResponse.Resources {
		if resource.Server != nil {
			response.Resources = append(response.Resources, &api.PaginatedResource{
				Resource: &api.PaginatedResource_Server{
					Server: newAPIServer(*resource.Server),
				},
				RequiresRequest: resource.RequiresRequest,
			})
		}
		if resource.Database != nil {
			response.Resources = append(response.Resources, &api.PaginatedResource{
				Resource: &api.PaginatedResource_Database{
					Database: newAPIDatabase(*resource.Database),
				},
				RequiresRequest: resource.RequiresRequest,
			})
		}
		if resource.Kube != nil {
			response.Resources = append(response.Resources, &api.PaginatedResource{
				Resource: &api.PaginatedResource_Kube{
					Kube: newAPIKube(*resource.Kube),
				},
				RequiresRequest: resource.RequiresRequest,
			})
		}
		if resource.App != nil {
			response.Resources = append(response.Resources, &api.PaginatedResource{
				Resource: &api.PaginatedResource_App{
					App: newAPIApp(*resource.App),
				},
				RequiresRequest: resource.RequiresRequest,
			})
		}
		if resource.SAMLIdPServiceProvider != nil {
			response.Resources = append(response.Resources, &api.PaginatedResource{
				Resource: &api.PaginatedResource_App{
					App: newSAMLIdPServiceProviderAPIApp(*resource.SAMLIdPServiceProvider),
				},
				RequiresRequest: resource.RequiresRequest,
			})
		}
	}

	return &response, nil
}
