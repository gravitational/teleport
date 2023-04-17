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

// GetServers accepts parameterized input to enable searching, sorting, and pagination
func (s *Handler) GetServers(ctx context.Context, req *api.GetServersRequest) (*api.GetServersResponse, error) {
	resp, err := s.DaemonService.GetServers(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := &api.GetServersResponse{
		TotalCount: int32(resp.TotalCount),
		StartKey:   resp.StartKey,
	}
	for _, srv := range resp.Servers {
		response.Agents = append(response.Agents, newAPIServer(srv))
	}

	return response, nil
}

func newAPIServer(server clusters.Server) *api.Server {
	apiLabels := APILabels{}
	serverLabels := server.GetStaticLabels()
	for name, value := range serverLabels {
		apiLabels = append(apiLabels, &api.Label{
			Name:  name,
			Value: value,
		})
	}

	serverCmdLabels := server.GetCmdLabels()
	for name, cmd := range serverCmdLabels {
		apiLabels = append(apiLabels, &api.Label{
			Name:  name,
			Value: cmd.GetResult(),
		})
	}

	sort.Sort(apiLabels)

	return &api.Server{
		Uri:      server.URI.String(),
		Tunnel:   server.GetUseTunnel(),
		Name:     server.GetName(),
		Hostname: server.GetHostname(),
		Addr:     server.GetAddr(),
		Labels:   apiLabels,
	}
}
