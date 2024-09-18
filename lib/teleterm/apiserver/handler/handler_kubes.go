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
	"fmt"
	"sort"

	"github.com/gravitational/trace"

	kubeproto "github.com/gravitational/teleport/api/gen/proto/go/teleport/kube/v1"
	"github.com/gravitational/teleport/api/types"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
)

// GetKubes accepts parameterized input to enable searching, sorting, and pagination
func (s *Handler) GetKubes(ctx context.Context, req *api.GetKubesRequest) (*api.GetKubesResponse, error) {
	resp, err := s.DaemonService.GetKubes(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := &api.GetKubesResponse{
		TotalCount: int32(resp.TotalCount),
		StartKey:   resp.StartKey,
	}
	for _, kube := range resp.Kubes {
		response.Agents = append(response.Agents, newAPIKube(kube))
	}

	return response, nil
}

// ListKubernetesResources accepts parameterized input to enable searching, sorting, and pagination
func (s *Handler) ListKubernetesResources(ctx context.Context, req *api.ListKubernetesResourcesRequest) (*api.ListKubernetesResourcesResponse, error) {
	fmt.Println("------- here")

	clusterURI, err := uri.Parse(req.GetClusterUri())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fmt.Println("------- here print clusterURI", clusterURI, req.Search)
	fmt.Println("---- parsed search: ", client.ParseSearchKeywords(req.GetSearch(), ' '))

	resp, err := s.DaemonService.ListKubernetesResources(ctx, clusterURI, &kubeproto.ListKubernetesResourcesRequest{
		ResourceType:      req.GetKubeResourceType(),
		SearchKeywords:    client.ParseSearchKeywords(req.GetSearch(), ' '),
		UseSearchAsRoles:  req.GetSearchAsRoles(),
		KubernetesCluster: req.GetKubeCluster(),
		Limit:             req.GetLimit(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := &api.ListKubernetesResourcesResponse{
		StartKey: resp.NextKey,
	}

	for _, kubeResource := range resp.Resources {
		response.Items = append(response.Items, newApiKubeResource(kubeResource, req.ClusterUri))
	}

	return response, nil
}

func newAPIKube(kube clusters.Kube) *api.Kube {
	apiLabels := APILabels{}
	for name, value := range kube.KubernetesCluster.GetStaticLabels() {
		apiLabels = append(apiLabels, &api.Label{
			Name:  name,
			Value: value,
		})
	}

	for name, cmd := range kube.KubernetesCluster.GetDynamicLabels() {
		apiLabels = append(apiLabels, &api.Label{
			Name:  name,
			Value: cmd.GetResult(),
		})
	}

	sort.Sort(apiLabels)

	return &api.Kube{
		Name:   kube.KubernetesCluster.GetName(),
		Uri:    kube.URI.String(),
		Labels: apiLabels,
	}
}

func newApiKubeResource(resource *types.KubernetesResourceV1, cluster string) *api.KubeResource {
	apiLabels := APILabels{}
	for name, value := range resource.GetStaticLabels() {
		apiLabels = append(apiLabels, &api.Label{
			Name:  name,
			Value: value,
		})
	}

	return &api.KubeResource{
		Kind:      resource.Kind,
		Name:      resource.GetName(),
		Labels:    apiLabels,
		Namespace: resource.Spec.Namespace,
		Cluster:   cluster,
	}
}
