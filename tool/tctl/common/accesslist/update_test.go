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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
)

func TestApplySpecFlags(t *testing.T) {
	tests := []struct {
		name   string
		cmd    Command
		assert func(t *testing.T, al *accesslist.AccessList)
	}{
		{
			name: "title",
			cmd:  Command{titleSet: true, title: "New Title"},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, "New Title", al.Spec.Title)
			},
		},
		{
			name: "description",
			cmd:  Command{descriptionSet: true, description: "New Desc"},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, "New Desc", al.Spec.Description)
			},
		},
		{
			name: "audit-frequency",
			cmd:  Command{auditFrequencySet: true, auditFrequency: 6},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, accesslist.SixMonths, al.Spec.Audit.Recurrence.Frequency)
			},
		},
		{
			name: "audit-day",
			cmd:  Command{auditDaySet: true, auditDay: 15},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, accesslist.FifteenthDayOfMonth, al.Spec.Audit.Recurrence.DayOfMonth)
			},
		},
		{
			name: "owner-grant-roles",
			cmd:  Command{ownerGrantRolesSet: true, ownerGrantRoles: "role1,role2"},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, []string{"role1", "role2"}, al.Spec.OwnerGrants.Roles)
			},
		},
		{
			name: "owner-grant-traits",
			cmd:  Command{ownerGrantTraitsSet: true, ownerGrantTraits: "team=apple,team=banana"},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, []string{"apple", "banana"}, al.Spec.OwnerGrants.Traits["team"])
			},
		},
		{
			name: "owner-required-roles",
			cmd:  Command{ownerRequiredRolesSet: true, ownerRequiredRoles: "role1,role2"},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, []string{"role1", "role2"}, al.Spec.OwnershipRequires.Roles)
			},
		},
		{
			name: "owner-required-traits",
			cmd:  Command{ownerRequiredTraitsSet: true, ownerRequiredTraits: "team=apple,team=banana"},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, []string{"apple", "banana"}, al.Spec.OwnershipRequires.Traits["team"])
			},
		},
		{
			name: "member-grant-roles",
			cmd:  Command{memberGrantRolesSet: true, memberGrantRoles: "role1,role2"},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, []string{"role1", "role2"}, al.Spec.Grants.Roles)
			},
		},
		{
			name: "member-grant-traits",
			cmd:  Command{memberGrantTraitsSet: true, memberGrantTraits: "team=apple,team=banana"},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, []string{"apple", "banana"}, al.Spec.Grants.Traits["team"])
			},
		},
		{
			name: "member-required-roles",
			cmd:  Command{memberRequiredRolesSet: true, memberRequiredRoles: "role1,role2"},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, []string{"role1", "role2"}, al.Spec.MembershipRequires.Roles)
			},
		},
		{
			name: "member-required-traits",
			cmd:  Command{memberRequiredTraitsSet: true, memberRequiredTraits: "team=apple,team=banana"},
			assert: func(t *testing.T, al *accesslist.AccessList) {
				require.Equal(t, []string{"apple", "banana"}, al.Spec.MembershipRequires.Traits["team"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			al := &accesslist.AccessList{}
			require.NoError(t, tt.cmd.applySpecFlags(al))
			tt.assert(t, al)
		})
	}
}

func TestApplyStandardAccessFlagsToRole(t *testing.T) {
	tests := []struct {
		name   string
		cmd    Command
		assert func(t *testing.T, allow types.RoleConditions)
	}{
		// Nodes
		{
			name: "node-labels",
			cmd:  Command{nodeLabelsSet: true, nodeLabels: "env=staging,env=test"},
			assert: func(t *testing.T, a types.RoleConditions) {
				require.Equal(t, []string{"staging", "test"}, []string(a.NodeLabels["env"]))
			},
		},
		{
			name: "logins",
			cmd:  Command{loginsSet: true, logins: "root,ubuntu"},
			assert: func(t *testing.T, a types.RoleConditions) {
				require.Equal(t, []string{"root", "ubuntu"}, a.Logins)
			},
		},
		// Dbs
		{
			name: "db-labels",
			cmd:  Command{dbLabelsSet: true, dbLabels: "env=staging,env=test"},
			assert: func(t *testing.T, a types.RoleConditions) {
				require.Equal(t, []string{"staging", "test"}, []string(a.DatabaseLabels["env"]))
			},
		},
		{
			name: "db-users",
			cmd:  Command{dbUsersSet: true, dbUsers: "user1,user2"},
			assert: func(t *testing.T, a types.RoleConditions) {
				require.Equal(t, []string{"user1", "user2"}, a.DatabaseUsers)
			},
		},
		{
			name: "db-names",
			cmd:  Command{dbNamesSet: true, dbNames: "postgres,sql"},
			assert: func(t *testing.T, a types.RoleConditions) {
				require.Equal(t, []string{"postgres", "sql"}, a.DatabaseNames)
			},
		},
		// Kubes
		{
			name: "kubernetes-labels",
			cmd:  Command{kubeLabelsSet: true, kubeLabels: "env=staging,env=test"},
			assert: func(t *testing.T, a types.RoleConditions) {
				require.Equal(t, []string{"staging", "test"}, []string(a.KubernetesLabels["env"]))
			},
		},
		{
			name: "kubernetes-users",
			cmd:  Command{kubeUsersSet: true, kubeUsers: "user1,user2"},
			assert: func(t *testing.T, a types.RoleConditions) {
				require.Equal(t, []string{"user1", "user2"}, a.KubeUsers)
			},
		},
		{
			name: "kubernetes-groups",
			cmd:  Command{kubeGroupsSet: true, kubeGroups: "group1,group2"},
			assert: func(t *testing.T, a types.RoleConditions) {
				require.Equal(t, []string{"group1", "group2"}, a.KubeGroups)
			},
		},
		// Apps
		{
			name: "app-labels",
			cmd:  Command{appLabelsSet: true, appLabels: "env=staging,env=test"},
			assert: func(t *testing.T, a types.RoleConditions) {
				require.Equal(t, []string{"staging", "test"}, []string(a.AppLabels["env"]))
			},
		},
		{
			name: "aws-role-arns",
			cmd:  Command{awsRoleARNsSet: true, awsRoleARNs: "arn1,arn2"},
			assert: func(t *testing.T, a types.RoleConditions) {
				require.Equal(t, []string{"arn1", "arn2"}, a.AWSRoleARNs)
			},
		},
		{
			name: "azure-identities",
			cmd:  Command{azureIdentitiesSet: true, azureIdentities: "azure1,azure2"},
			assert: func(t *testing.T, a types.RoleConditions) {
				require.Equal(t, []string{"azure1", "azure2"}, a.AzureIdentities)
			},
		},
		{
			name: "gcp-service-accounts",
			cmd:  Command{gcpServiceAccountsSet: true, gcpServiceAccounts: "gcp1,gpc2"},
			assert: func(t *testing.T, a types.RoleConditions) {
				require.Equal(t, []string{"gcp1", "gpc2"}, a.GCPServiceAccounts)
			},
		},
		{
			name: "mcp-tools",
			cmd:  Command{mcpToolsSet: true, mcpTools: "tool1,tool2"},
			assert: func(t *testing.T, a types.RoleConditions) {
				require.NotNil(t, a.MCP)
				require.Equal(t, []string{"tool1", "tool2"}, a.MCP.Tools)
			},
		},
		// Windows
		{
			name: "windows-labels",
			cmd:  Command{windowsLabelsSet: true, windowsLabels: "env=staging,env=test"},
			assert: func(t *testing.T, a types.RoleConditions) {
				require.Equal(t, []string{"staging", "test"}, []string(a.WindowsDesktopLabels["env"]))
			},
		},
		{
			name: "windows-logins",
			cmd:  Command{windowsLoginsSet: true, windowsLogins: "login1,login2"},
			assert: func(t *testing.T, a types.RoleConditions) {
				require.Equal(t, []string{"login1", "login2"}, a.WindowsDesktopLogins)
			},
		},
		// GitHub
		{
			name: "github-orgs",
			cmd:  Command{gitHubOrgsSet: true, gitHubOrgs: "apple,banana"},
			assert: func(t *testing.T, a types.RoleConditions) {
				require.Equal(t, []types.GitHubPermission{{Organizations: []string{"apple", "banana"}}}, a.GitHubPermissions)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var allow types.RoleConditions
			require.NoError(t, tt.cmd.applyStandardAccessFlagsToRole(&allow))
			tt.assert(t, allow)
		})
	}
}

func TestApplyStandardAccessFlagsToRole_ClearingFields(t *testing.T) {
	t.Run("clears when mcp-tools is set to empty", func(t *testing.T) {
		allow := types.RoleConditions{MCP: &types.MCPPermissions{Tools: []string{"apple"}}}
		cmd := Command{mcpToolsSet: true, mcpTools: ""}
		require.NoError(t, cmd.applyStandardAccessFlagsToRole(&allow))
		require.Nil(t, allow.MCP)
	})
	t.Run("clears when github-orgs is set to empty", func(t *testing.T) {
		allow := types.RoleConditions{GitHubPermissions: []types.GitHubPermission{{Organizations: []string{"apple"}}}}
		cmd := Command{gitHubOrgsSet: true, gitHubOrgs: ""}
		require.NoError(t, cmd.applyStandardAccessFlagsToRole(&allow))
		require.Nil(t, allow.GitHubPermissions)
	})
}

func TestApplyAWSICFlagsToRole(t *testing.T) {
	t.Run("sets assignments", func(t *testing.T) {
		cmd := Command{awsicAssignments: "1234:arn:aws:sso:::permissionSet/test,5678:arn:aws:sso:::permissionSet/test2"}
		var allow types.RoleConditions
		require.NoError(t, cmd.applyAWSICFlagsToRole(&allow))
		require.Equal(t, awsIcAppLabel, allow.AppLabels)
		require.Len(t, allow.AccountAssignments, 2)
		require.Equal(t,
			types.IdentityCenterAccountAssignment{
				PermissionSet: "arn:aws:sso:::permissionSet/test",
				Account:       "1234",
			},
			allow.AccountAssignments[0])
		require.Equal(t,
			types.IdentityCenterAccountAssignment{
				PermissionSet: "arn:aws:sso:::permissionSet/test2",
				Account:       "5678",
			},
			allow.AccountAssignments[1])
	})
	t.Run("clears when empty", func(t *testing.T) {
		allow := types.RoleConditions{
			AppLabels:          awsIcAppLabel,
			AccountAssignments: []types.IdentityCenterAccountAssignment{{}},
		}
		cmd := Command{awsicAssignments: ""}
		require.NoError(t, cmd.applyAWSICFlagsToRole(&allow))
		require.Empty(t, allow)
	})
}
