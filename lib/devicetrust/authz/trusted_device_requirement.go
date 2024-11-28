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
