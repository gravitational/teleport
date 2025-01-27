// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package vnet

import (
	"context"

	"github.com/gravitational/trace"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/vnet/v1"
	"github.com/gravitational/teleport/lib/vnet/diag"
)

// RunDiagnostics runs a set of heuristics to determine if VNet actually works on the device, that
// is receives network traffic and DNS queries. RunDiagnostics requires VNet to be started.
func (s *Service) RunDiagnostics(ctx context.Context, req *api.RunDiagnosticsRequest) (*api.RunDiagnosticsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status != statusRunning {
		return nil, trace.CompareFailed("VNet is not running")
	}

	if s.networkStackInfo.IfaceName == "" {
		return nil, trace.BadParameter("no interface name, this is a bug")
	}

	routeConflictDiag, err := diag.NewRouteConflictDiag(&diag.RouteConflictConfig{
		VnetIfaceName: s.networkStackInfo.IfaceName,
		Routing:       &diag.DarwinRouting{},
		Interfaces:    &diag.NetInterfaces{},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rcs, err := routeConflictDiag.Run(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, rc := range rcs {
		log.InfoContext(ctx, "Found conflicting route", "route", rc)
	}

	return &api.RunDiagnosticsResponse{}, nil
}
