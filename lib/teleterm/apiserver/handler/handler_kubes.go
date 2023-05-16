// Copyright 2021 Gravitational, Inc
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
	"sort"

	"github.com/gravitational/trace"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
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
