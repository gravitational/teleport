/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package services

import (
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// ValidateTrustedCluster checks and sets Trusted Cluster defaults
func ValidateTrustedCluster(tc types.TrustedCluster, allowEmptyRolesOpts ...bool) error {
	if err := CheckAndSetDefaults(tc); err != nil {
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

// RoleMapToString prints user friendly representation of role mapping
func RoleMapToString(r types.RoleMap) string {
	values, err := parseRoleMap(r)
	if err != nil {
		return fmt.Sprintf("<failed to parse: %v", err)
	}
	if len(values) != 0 {
		return fmt.Sprintf("%v", values)
	}
	return "<empty>"
}

func parseRoleMap(r types.RoleMap) (map[string][]string, error) {
	directMatch := make(map[string][]string)
	for i := range r {
		roleMap := r[i]
		if roleMap.Remote == "" {
			return nil, trace.BadParameter("missing 'remote' parameter for role_map")
		}
		_, err := utils.ReplaceRegexp(roleMap.Remote, "", "")
		if trace.IsBadParameter(err) {
			return nil, trace.BadParameter("failed to parse 'remote' parameter for role_map: %v", err.Error())
		}
		if len(roleMap.Local) == 0 {
			return nil, trace.BadParameter("missing 'local' parameter for 'role_map'")
		}
		for _, local := range roleMap.Local {
			if local == "" {
				return nil, trace.BadParameter("missing 'local' property of 'role_map' entry")
			}
			if local == types.Wildcard {
				return nil, trace.BadParameter("wildcard value is not supported for 'local' property of 'role_map' entry")
			}
		}
		_, ok := directMatch[roleMap.Remote]
		if ok {
			return nil, trace.BadParameter("remote role '%v' match is already specified", roleMap.Remote)
		}
		directMatch[roleMap.Remote] = roleMap.Local
	}
	return directMatch, nil
}

// MapRoles maps local roles to remote roles
func MapRoles(r types.RoleMap, remoteRoles []string) ([]string, error) {
	_, err := parseRoleMap(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	seen := make(map[string]struct{})
	var outRoles []string
	// when no remote roles are specified, assume that
	// there is a single empty remote role (that should match wildcards)
	if len(remoteRoles) == 0 {
		remoteRoles = []string{""}
	}
	for _, mapping := range r {
		expression := mapping.Remote
		for _, remoteRole := range remoteRoles {
			// never map default implicit role, it is always
			// added by default
			if remoteRole == constants.DefaultImplicitRole {
				continue
			}
			for _, replacementRole := range mapping.Local {
				replacement, err := utils.ReplaceRegexp(expression, replacementRole, remoteRole)
				switch {
				case err == nil:
					// empty replacement can occur when $2 expand refers
					// to non-existing capture group in match expression
					if replacement == "" {
						continue
					}
					if _, ok := seen[replacement]; ok {
						continue
					}
					seen[replacement] = struct{}{}
					outRoles = append(outRoles, replacement)
				case trace.IsNotFound(err):
					continue
				default:
					return nil, trace.Wrap(err)
				}
			}
		}
	}
	return outRoles, nil
}

// UnmarshalTrustedCluster unmarshals the TrustedCluster resource from JSON.
func UnmarshalTrustedCluster(bytes []byte, opts ...MarshalOption) (types.TrustedCluster, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var trustedCluster types.TrustedClusterV2

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	if err := utils.FastUnmarshal(bytes, &trustedCluster); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	// DELETE IN(7.0)
	// temporarily allow to read trusted cluster with no role map
	// until users migrate from 6.0 OSS that had no role map present
	const allowEmptyRoleMap = true
	if err = ValidateTrustedCluster(&trustedCluster, allowEmptyRoleMap); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		trustedCluster.SetResourceID(cfg.ID)
	}
	if cfg.Revision != "" {
		trustedCluster.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		trustedCluster.SetExpiry(cfg.Expires)
	}
	return &trustedCluster, nil
}

// MarshalTrustedCluster marshals the TrustedCluster resource to JSON.
func MarshalTrustedCluster(trustedCluster types.TrustedCluster, opts ...MarshalOption) ([]byte, error) {
	// DELETE IN(7.0)
	// temporarily allow to read trusted cluster with no role map
	// until users migrate from 6.0 OSS that had no role map present
	const allowEmptyRoleMap = true
	if err := ValidateTrustedCluster(trustedCluster, allowEmptyRoleMap); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch trustedCluster := trustedCluster.(type) {
	case *types.TrustedClusterV2:
		return utils.FastMarshal(maybeResetProtoResourceID(cfg.PreserveResourceID, trustedCluster))
	default:
		return nil, trace.BadParameter("unrecognized trusted cluster version %T", trustedCluster)
	}
}
