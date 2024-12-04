// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package authz

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	dtconfig "github.com/gravitational/teleport/lib/devicetrust/config"
)

// CalculateTrustedDeviceRequirement calculates the requirement based on
// the cluster config and user roles.
// It checks the cluster requirement using the enforcement mode, disregarding the
// provenance of the binary if the mode is set.
func CalculateTrustedDeviceRequirement(
	dt *types.DeviceTrust,
	getRoles func() ([]types.Role, error),
) (types.TrustedDeviceRequirement, error) {
	// Required by cluster mode?
	if dtconfig.GetEnforcementMode(dt) == constants.DeviceTrustModeRequired {
		return types.TrustedDeviceRequirement_TRUSTED_DEVICE_REQUIREMENT_REQUIRED, nil
	}

	// Required by roles?
	roles, err := getRoles()
	if err != nil {
		return types.TrustedDeviceRequirement_TRUSTED_DEVICE_REQUIREMENT_UNSPECIFIED, trace.Wrap(err)
	}
	for _, role := range roles {
		if role.GetOptions().DeviceTrustMode == constants.DeviceTrustModeRequired {
			return types.TrustedDeviceRequirement_TRUSTED_DEVICE_REQUIREMENT_REQUIRED, nil
		}
	}

	return types.TrustedDeviceRequirement_TRUSTED_DEVICE_REQUIREMENT_NOT_REQUIRED, nil
}
