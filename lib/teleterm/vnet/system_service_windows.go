// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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
	"github.com/gravitational/teleport/lib/vnet"
)

// GetSystemService verifies the existence of the VNet system service, which is installed only in per-machine setups.
func (s *Service) GetSystemService(_ context.Context, _ *api.GetWindowsSystemServiceRequest) (*api.GetWindowsSystemServiceResponse, error) {
	err := vnet.VerifyServiceInstalled()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &api.GetWindowsSystemServiceResponse{}, nil
}
