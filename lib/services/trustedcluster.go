package services

import (
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/modules"

	"github.com/gravitational/trace"
)

// CheckAndSetTrustedClusterDefaults checks and sets Trusted Cluster defaults
func CheckAndSetTrustedClusterDefaults(tc TrustedCluster) error {
	if err := tc.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// we are not mentioning Roles parameter because we are deprecating it
	if len(tc.GetRoles()) == 0 && len(tc.GetRoleMap()) == 0 {
		if err := modules.GetModules().EmptyRolesHandler(); err != nil {
			return trace.Wrap(err)
		}
		// OSS teleport uses 'admin' by default:
		tc.SetRoleMap(RoleMap{
			RoleMapping{
				Remote: teleport.AdminRoleName,
				Local:  []string{teleport.AdminRoleName},
			},
		})
	}

	return nil
}
