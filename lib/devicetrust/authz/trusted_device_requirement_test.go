package authz_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/devicetrust/authz"
)

func TestCalculateTrustedDeviceRequirement(t *testing.T) {
	deviceTrustOptionalRole, err := types.NewRole("device-trust-optional", types.RoleSpecV6{Options: types.RoleOptions{DeviceTrustMode: constants.DeviceTrustModeOptional}})
	assert.NoError(t, err)
	deviceTrustRequiredRole, err := types.NewRole("device-trust-required", types.RoleSpecV6{Options: types.RoleOptions{DeviceTrustMode: constants.DeviceTrustModeRequired}})
	assert.NoError(t, err)

	tests := []struct {
		name              string
		dt                *types.DeviceTrust
		roles             []types.Role
		expectRequirement types.TrustedDeviceRequirement
	}{
		{
			name: "not required by cluster or by roles",
			dt: &types.DeviceTrust{
				Mode: constants.DeviceTrustModeOptional,
			},
			roles:             []types.Role{deviceTrustOptionalRole},
			expectRequirement: types.TrustedDeviceRequirement_TRUSTED_DEVICE_REQUIREMENT_NOT_REQUIRED,
		},
		{
			name: "required by cluster but not by roles",
			dt: &types.DeviceTrust{
				Mode: constants.DeviceTrustModeRequired,
			},
			roles:             []types.Role{deviceTrustOptionalRole},
			expectRequirement: types.TrustedDeviceRequirement_TRUSTED_DEVICE_REQUIREMENT_REQUIRED,
		},
		{
			name: "required by role but not by cluster",
			dt: &types.DeviceTrust{
				Mode: constants.DeviceTrustModeOptional,
			},
			roles:             []types.Role{deviceTrustRequiredRole, deviceTrustOptionalRole},
			expectRequirement: types.TrustedDeviceRequirement_TRUSTED_DEVICE_REQUIREMENT_REQUIRED,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requirement, err := authz.CalculateTrustedDeviceRequirement(test.dt, func() ([]types.Role, error) {
				return test.roles, nil
			})
			assert.NoError(t, err)

			assert.Equal(t, test.expectRequirement, requirement)
		})
	}
}
