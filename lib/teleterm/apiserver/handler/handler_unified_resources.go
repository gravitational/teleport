// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
			})
		}
		if resource.Database != nil {
			response.Resources = append(response.Resources, &api.PaginatedResource{
				Resource: &api.PaginatedResource_Database{
					Database: newAPIDatabase(*resource.Database),
				},
			})
		}
		if resource.Kube != nil {
			response.Resources = append(response.Resources, &api.PaginatedResource{
				Resource: &api.PaginatedResource_Kube{
					Kube: newAPIKube(*resource.Kube),
				},
			})
		}
	}

	return &response, nil
}
