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

package accesslist

import (
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/lib/accesslists/preset"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// CLI-facing access type values (what the user sees and uses,
	// which aligns with terms used in docs).
	accessTypeLongTerm  = "standing"
	accessTypeShortTerm = "request-based"

	standardRolePrefixName = "access-standard"
	awsicRolePrefixName    = "access-awsic"
)

var awsIcAppLabel = types.Labels{types.OriginLabel: []string{common.OriginAWSIdentityCenter}}

func buildAWSICAccountAssignments(awsicAssignments string) ([]types.IdentityCenterAccountAssignment, error) {
	var aa []types.IdentityCenterAccountAssignment
	for _, a := range utils.SplitIdentifiers(awsicAssignments) {
		account, permSet, ok := strings.Cut(a, ":")
		if !ok {
			return nil, trace.BadParameter("--aws-ic-assignments: %q is not in 'accountID:permissionSetARN' format", a)
		}
		aa = append(aa, types.IdentityCenterAccountAssignment{
			Account:       strings.TrimSpace(account),
			PermissionSet: strings.TrimSpace(permSet),
		})
	}
	return aa, nil
}

// buildRole returns a roleV6. "roleName" is overwritten in the backend by appending
// the role name with the access list ID, making these roles unique e.g.:
// "access-standard" becomes access-standard-acl-preset-<access-list-uuid>
func buildRole(roleName string, allow types.RoleConditions) (*types.RoleV6, error) {
	spec := types.RoleSpecV6{
		Allow: allow,
	}
	role, err := types.NewRole(roleName, spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	r, ok := role.(*types.RoleV6)
	if !ok {
		return nil, trace.BadParameter("unexpected role type %T", role)
	}
	return r, nil
}

// accessType maps the backend value into the user facing accessType value.
func accessType(presetType string) string {
	switch presetType {
	case string(preset.LongTermPresetType):
		return accessTypeLongTerm
	case string(preset.ShortTermPresetType):
		return accessTypeShortTerm
	}
	return ""
}
