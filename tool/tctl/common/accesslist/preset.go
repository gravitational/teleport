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
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/parse"
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

type applyAccessFlagsToRole func(allow *types.RoleConditions) error

// applyStandardAccessFlagsToRole modifies the standard role's allow block.
// Empty values clear the field, unset flags leave field alone.
func (c *Command) applyStandardAccessFlagsToRole(allow *types.RoleConditions) error {
	// Nodes
	if c.nodeLabelsSet {
		labels, err := parse.MultiValueLabelSelectorSpec(c.nodeLabels)
		if err != nil {
			return trace.Wrap(err, "--node-labels")
		}
		allow.NodeLabels = types.ToLabels(labels)
	}
	if c.loginsSet {
		allow.Logins = utils.SplitIdentifiers(c.logins)
	}

	// Dbs
	if c.dbLabelsSet {
		labels, err := parse.MultiValueLabelSelectorSpec(c.dbLabels)
		if err != nil {
			return trace.Wrap(err, "--db-labels")
		}
		allow.DatabaseLabels = types.ToLabels(labels)
	}
	if c.dbUsersSet {
		allow.DatabaseUsers = utils.SplitIdentifiers(c.dbUsers)
	}
	if c.dbNamesSet {
		allow.DatabaseNames = utils.SplitIdentifiers(c.dbNames)
	}

	// Kubes
	if c.kubeLabelsSet {
		labels, err := parse.MultiValueLabelSelectorSpec(c.kubeLabels)
		if err != nil {
			return trace.Wrap(err, "--kubernetes-labels")
		}
		allow.KubernetesLabels = types.ToLabels(labels)
	}
	if c.kubeUsersSet {
		allow.KubeUsers = utils.SplitIdentifiers(c.kubeUsers)
	}
	if c.kubeGroupsSet {
		allow.KubeGroups = utils.SplitIdentifiers(c.kubeGroups)
	}

	// Apps
	if c.appLabelsSet {
		labels, err := parse.MultiValueLabelSelectorSpec(c.appLabels)
		if err != nil {
			return trace.Wrap(err, "--app-labels")
		}
		allow.AppLabels = types.ToLabels(labels)
	}
	if c.awsRoleARNsSet {
		allow.AWSRoleARNs = utils.SplitIdentifiers(c.awsRoleARNs)
	}
	if c.azureIdentitiesSet {
		allow.AzureIdentities = utils.SplitIdentifiers(c.azureIdentities)
	}
	if c.gcpServiceAccountsSet {
		allow.GCPServiceAccounts = utils.SplitIdentifiers(c.gcpServiceAccounts)
	}
	if c.mcpToolsSet {
		tools := utils.SplitIdentifiers(c.mcpTools)
		if len(tools) == 0 {
			allow.MCP = nil
		} else {
			allow.MCP = &types.MCPPermissions{Tools: tools}
		}
	}

	// Windows
	if c.windowsLabelsSet {
		labels, err := parse.MultiValueLabelSelectorSpec(c.windowsLabels)
		if err != nil {
			return trace.Wrap(err, "--windows-labels")
		}
		allow.WindowsDesktopLabels = types.ToLabels(labels)
	}
	if c.windowsLoginsSet {
		allow.WindowsDesktopLogins = utils.SplitIdentifiers(c.windowsLogins)
	}

	// GitHub
	if c.gitHubOrgsSet {
		orgs := utils.SplitIdentifiers(c.gitHubOrgs)
		if len(orgs) == 0 {
			allow.GitHubPermissions = nil
		} else {
			allow.GitHubPermissions = []types.GitHubPermission{{Organizations: orgs}}
		}
	}
	return nil
}

// applyAWSICFlagsToRole modifies the AWS IC role's allow block.
// Empty values clear the whole allow spec since this role is specific to awsic,
// unset flags leave fields alone.
func (c *Command) applyAWSICFlagsToRole(allow *types.RoleConditions) error {
	if c.awsicAssignments == "" {
		*allow = types.RoleConditions{}
	} else {
		allow.AppLabels = awsIcAppLabel
		aa, err := buildAWSICAccountAssignments(c.awsicAssignments)
		if err != nil {
			return trace.Wrap(err)
		}
		allow.AccountAssignments = aa
	}
	return nil
}

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
func accessType(preset string) string {
	switch preset {
	case string(accesslist.LongTermPresetType):
		return accessTypeLongTerm
	case string(accesslist.ShortTermPresetType):
		return accessTypeShortTerm
	}
	return ""
}

// presetType converts user facing accessType value to the expected backend value.
func presetType(accessType string) string {
	switch accessType {
	case accessTypeLongTerm:
		return string(accesslist.LongTermPresetType)
	case accessTypeShortTerm:
		return string(accesslist.ShortTermPresetType)
	}
	return ""
}
