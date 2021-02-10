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

package auth

import (
	"github.com/gravitational/trace"
)

// ValidateTrustedCluster checks and sets Trusted Cluster defaults
func ValidateTrustedCluster(tc TrustedCluster, allowEmptyRolesOpts ...bool) error {
	if err := tc.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// DELETE IN (7.0)
	// This flag is used to allow reading trusted clusters with no role map.
	// This was possible in OSS before 6.0 release.
	allowEmptyRoles := false
	if len(allowEmptyRolesOpts) != 0 {
		allowEmptyRoles = allowEmptyRolesOpts[0]
	}
	// we are not mentioning Roles parameter because we are deprecating it
	if len(tc.GetRoles()) == 0 && len(tc.GetRoleMap()) == 0 {
		if !allowEmptyRoles {
			return trace.BadParameter("missing 'role_map' parameter")
		}
	}

	if _, err := parseRoleMap(tc.GetRoleMap()); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
