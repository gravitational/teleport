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
	"sort"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/ui"
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

// ListKubernetesResourcesRequest defines a request to retrieve kube resources paginated.
// Only one type of kube resource can be retrieved per request (eg: namespace, pods, secrets, etc.)
func (s *Handler) ListKubernetesResources(ctx context.Context, req *api.ListKubernetesResourcesRequest) (*api.ListKubernetesResourcesResponse, error) {
	clusterURI, err := uri.Parse(req.GetClusterUri())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resources, err := s.DaemonService.ListKubernetesResources(ctx, clusterURI, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := &api.ListKubernetesResourcesResponse{}

	for _, resource := range resources {
		kubeResource, ok := resource.(*types.KubernetesResourceV1)
		if !ok {
			return nil, trace.BadParameter("expected resource type %T, got %T", types.KubernetesResourceV1{}, resource)
		}
		response.Resources = append(response.Resources, newApiKubeResource(kubeResource, req.GetKubernetesCluster(), clusterURI))
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

func newApiKubeResource(resource *types.KubernetesResourceV1, kubeCluster string, resourceURI uri.ResourceURI) *api.KubeResource {
	uiLabels := ui.MakeLabelsWithoutInternalPrefixes(resource.GetStaticLabels())
	apiLabels := APILabels{}
	for _, uiLabel := range uiLabels {
		apiLabels = append(apiLabels, &api.Label{
			Name:  uiLabel.Name,
			Value: uiLabel.Value,
		})
	}

	return &api.KubeResource{
		Uri:       resourceURI.AppendKube(kubeCluster).AppendKubeResourceNamespace(resource.GetName()).String(),
		Kind:      resource.GetKind(),
		Name:      resource.GetName(),
		Labels:    apiLabels,
		Namespace: resource.Spec.Namespace,
		Cluster:   kubeCluster,
	}
}
