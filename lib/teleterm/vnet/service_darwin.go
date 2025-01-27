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
	diagv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/diag/v1"
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

	if s.networkStackInfo.IPv6Prefix == "" {
		return nil, trace.BadParameter("no IPv6 prefix, this is a bug")
	}

	nsa := &diagv1.NetworkStackAttempt{}
	if ns, err := s.getNetworkStack(ctx); err != nil {
		nsa.Status = diagv1.CheckAttemptStatus_CHECK_ATTEMPT_STATUS_ERROR
		nsa.Error = err.Error()
	} else {
		nsa.Status = diagv1.CheckAttemptStatus_CHECK_ATTEMPT_STATUS_OK
		nsa.NetworkStack = ns
	}

	routeConflictDiag, err := diag.NewRouteConflictDiag(&diag.RouteConflictConfig{
		VnetIfaceName: s.networkStackInfo.IfaceName,
		Routing:       &diag.DarwinRouting{},
		Interfaces:    &diag.NetInterfaces{},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	diagnostician := diag.Diagnostician{}
	report, err := diagnostician.GenerateReport(ctx, diag.ReportPrerequisites{
		Clock:               s.cfg.Clock,
		NetworkStackAttempt: nsa,
		DiagChecks:          []diag.DiagCheck{routeConflictDiag},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.RunDiagnosticsResponse{
		Report: report,
	}, nil
}

func (s *Service) getNetworkStack(ctx context.Context) (*diagv1.NetworkStack, error) {
	profileNames, err := s.cfg.DaemonService.ListProfileNames()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dnsZones, cidrRanges := s.listDNSZonesAndCIDRRanges(ctx, profileNames)

	return &diagv1.NetworkStack{
		InterfaceName:  s.networkStackInfo.IfaceName,
		Ipv6Prefix:     s.networkStackInfo.IPv6Prefix,
		Ipv4CidrRanges: cidrRanges,
		DnsZones:       dnsZones,
	}, nil
}
