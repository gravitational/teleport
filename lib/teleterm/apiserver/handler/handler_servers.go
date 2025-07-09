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

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/ui"
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
	serverLabels := server.GetStaticLabels()
	serverCmdLabels := server.GetCmdLabels()
	apiLabels := makeAPILabels(
		ui.MakeLabelsWithoutInternalPrefixes(serverLabels, ui.TransformCommandLabels(serverCmdLabels)),
	)

	return &api.Server{
		Uri:      server.URI.String(),
		Tunnel:   server.GetUseTunnel(),
		Name:     server.GetName(),
		Hostname: server.GetHostname(),
		Addr:     server.GetAddr(),
		SubKind:  server.GetSubKind(),
		Labels:   apiLabels,
	}
}
