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

	api "github.com/gravitational/teleport/lib/teleterm/api/protogen/golang/v1"
	"github.com/gravitational/teleport/lib/teleterm/clusters"

	"github.com/gravitational/trace"
)

// ListKubes lists kubernetes clusters
func (s *Handler) ListKubes(ctx context.Context, req *api.ListKubesRequest) (*api.ListKubesResponse, error) {
	kubes, err := s.DaemonService.ListKubes(ctx, req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := &api.ListKubesResponse{}
	for _, k := range kubes {
		response.Kubes = append(response.Kubes, newAPIKube(k))
	}

	return response, nil
}

func newAPIKube(kube clusters.Kube) *api.Kube {
	apiLabels := APILabels{}
	for name, value := range kube.StaticLabels {
		apiLabels = append(apiLabels, &api.Label{
			Name:  name,
			Value: value,
		})
	}

	for name, cmd := range kube.DynamicLabels {
		apiLabels = append(apiLabels, &api.Label{
			Name:  name,
			Value: cmd.GetResult(),
		})
	}

	sort.Sort(apiLabels)

	return &api.Kube{
		Name:   kube.Name,
		Uri:    kube.URI.String(),
		Labels: apiLabels,
	}
}
