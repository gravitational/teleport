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
	"errors"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/vnet/v1"
	"github.com/gravitational/teleport/lib/vnet"
	"github.com/gravitational/teleport/lib/vnet/diag"
)

func (s *Service) platformDiagChecks(ctx context.Context) ([]diag.DiagCheck, error) {
	routeConflictDiag, err := diag.NewRouteConflictDiag(&diag.RouteConflictConfig{
		VnetIfaceName: s.networkStackInfo.InterfaceName,
		Routing:       &diag.WindowsRouting{},
		Interfaces:    &diag.NetInterfaces{},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sshDiag, err := diag.NewSSHDiag(&diag.SSHConfig{
		ProfilePath: s.cfg.profilePath,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return []diag.DiagCheck{
		routeConflictDiag,
		sshDiag,
	}, nil
}

// CheckPreRunRequirements verifies the existence of the VNet system service, which is installed only in per-machine setups.
func (s *Service) CheckPreRunRequirements(_ context.Context, _ *api.CheckPreRunRequirementsRequest) (*api.CheckPreRunRequirementsResponse, error) {
	err := vnet.VerifyServiceInstalled()
	if err == nil {
		return &api.CheckPreRunRequirementsResponse{
			PlatformStatus: &api.CheckPreRunRequirementsResponse_WindowsSystemServiceStatus{
				WindowsSystemServiceStatus: api.WindowsSystemServiceStatus_WINDOWS_SYSTEM_SERVICE_STATUS_OK,
			},
		}, nil
	}

	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return &api.CheckPreRunRequirementsResponse{
			PlatformStatus: &api.CheckPreRunRequirementsResponse_WindowsSystemServiceStatus{
				WindowsSystemServiceStatus: api.WindowsSystemServiceStatus_WINDOWS_SYSTEM_SERVICE_STATUS_DOES_NOT_EXIST,
			},
		}, nil

	}
	return nil, trace.Wrap(err)
}
