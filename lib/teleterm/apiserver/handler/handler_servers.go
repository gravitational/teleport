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

// ListServers lists servers
func (s *Handler) ListServers(ctx context.Context, req *api.ListServersRequest) (*api.ListServersResponse, error) {
	servers, err := s.DaemonService.ListServers(ctx, req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := &api.ListServersResponse{}
	for _, srv := range servers {
		response.Servers = append(response.Servers, newAPIServer(srv))
	}

	return response, nil
}

func newAPIServer(server clusters.Server) *api.Server {
	apiLabels := APILabels{}
	serverLabels := server.GetLabels()
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
