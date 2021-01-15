/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/modules"

	"github.com/gravitational/trace"
)

// ValidateTrustedCluster checks and sets Trusted Cluster defaults
func ValidateTrustedCluster(tc TrustedCluster) error {
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
