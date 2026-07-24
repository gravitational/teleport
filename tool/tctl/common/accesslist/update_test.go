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
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/lib/accesslists/preset"
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

func TestValidateStandardRoleSpec(t *testing.T) {
	emptyRole, err := getEmptyRoleV6("some-role-name", types.V8)
	require.NoError(t, err)

	tests := []struct {
		name                     string
		allow                    types.RoleConditions
		deny                     types.RoleConditions
		wantErrContains          string
		wantUnmanagedAllowFields []string
	}{
		{
			name:  "valid empty role conditions",
			allow: emptyRole.Spec.Allow,
			deny:  emptyRole.Spec.Deny,
		},
		{
			name: "all valid allow fields",
			allow: func() types.RoleConditions {
				allow := emptyRole.Spec.Allow
				allow.NodeLabels = types.Labels{"node": {"label"}}
				allow.Logins = []string{"nodelogins"}
				allow.DatabaseLabels = types.Labels{"db": {"label"}}
				allow.DatabaseUsers = []string{"dbuser"}
				allow.DatabaseNames = []string{"dbname"}
				allow.KubernetesLabels = types.Labels{"kube": {"label"}}
				allow.KubeUsers = []string{"kubeuser"}
				allow.KubeGroups = []string{"kubegroup"}
				allow.KubernetesResources = []types.KubernetesResource{{Kind: "pods", Namespace: "*", Name: "*"}}
				allow.AppLabels = types.Labels{"app": {"label"}}
				allow.AWSRoleARNs = []string{"arn:aws:iam::123456789012:role/example"}
				allow.AzureIdentities = []string{"azureidentity"}
				allow.GCPServiceAccounts = []string{"gcpserviceaccts"}
				allow.MCP = &types.MCPPermissions{Tools: []string{"mcptools"}}
				allow.WindowsDesktopLabels = types.Labels{"windows": {"labels"}}
				allow.WindowsDesktopLogins = []string{"windowslogins"}
				allow.GitHubPermissions = []types.GitHubPermission{{Organizations: []string{"githuborg"}}}
				return allow
			}(),
			deny: emptyRole.Spec.Deny,
		},
		{
			name: "single invalid field",
			allow: func() types.RoleConditions {
				allow := emptyRole.Spec.Allow
				allow.NodeLabelsExpression = `labels["env"] == "testing"`
				return allow
			}(),
			deny:            emptyRole.Spec.Deny,
			wantErrContains: "allow fields not supported by this update: node_labels_expression",
		},
		{
			name: "multiple invalid fields",
			allow: func() types.RoleConditions {
				allow := emptyRole.Spec.Allow
				allow.AppLabelsExpression = `labels["env"] == "test"`
				allow.NodeLabelsExpression = `labels["env"] == "test"`
				allow.Request = &types.AccessRequestConditions{
					Reason: &types.AccessRequestConditionsReason{},
				}
				allow.DatabaseServiceLabels = types.Labels{"env": []string{"test"}}
				// This is considered "empty" and should not be part of the wantUnmanagedAllowFields
				allow.ReviewRequests = &types.AccessReviewConditions{}
				return allow
			}(),
			deny:                     emptyRole.Spec.Deny,
			wantUnmanagedAllowFields: []string{"request", "db_service_labels", "node_labels_expression", "app_labels_expression"},
		},
		{
			name:  "invalid deny",
			allow: emptyRole.Spec.Allow,
			deny: func() types.RoleConditions {
				deny := emptyRole.Spec.Deny
				deny.Logins = []string{"root"}
				return deny
			}(),
			wantErrContains: "deny fields not supported by this update",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStandardRoleSpec("some-role-name", emptyRole, tt.allow, tt.deny)
			switch {
			case len(tt.wantUnmanagedAllowFields) > 0:
				require.True(t, trace.IsBadParameter(err))
				for _, field := range tt.wantUnmanagedAllowFields {
					require.Contains(t, err.Error(), field)
				}
			case tt.wantErrContains != "":
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), tt.wantErrContains)
			default:
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateAWSICRoleSpec(t *testing.T) {
	emptyRole, err := getEmptyRoleV6("some-role-name", types.V8)
	require.NoError(t, err)

	accountAssignments := []types.IdentityCenterAccountAssignment{
		{Account: "1234", PermissionSet: "arn:aws:sso:::permissionSet/test"},
	}
	validAllow := func() types.RoleConditions {
		allow := emptyRole.Spec.Allow
		allow.AppLabels = awsIcAppLabel
		allow.AccountAssignments = accountAssignments
		return allow
	}()

	tests := []struct {
		name            string
		allow           types.RoleConditions
		deny            types.RoleConditions
		wantErrContains string
	}{
		{
			name:  "valid fields",
			allow: validAllow,
			deny:  emptyRole.Spec.Deny,
		},
		{
			name: "valid label only",
			allow: func() types.RoleConditions {
				allow := emptyRole.Spec.Allow
				allow.AppLabels = awsIcAppLabel
				return allow
			}(),
			deny: emptyRole.Spec.Deny,
		},
		{
			name:  "valid empty allow",
			allow: emptyRole.Spec.Allow,
			deny:  emptyRole.Spec.Deny,
		},
		{
			name: "invalid label (hard coded by tctl)",
			allow: func() types.RoleConditions {
				allow := emptyRole.Spec.Allow
				allow.AppLabels = types.Labels{
					types.OriginLabel: []string{"aws-identity-center"},
					"offending":       []string{"label"},
				}
				return allow
			}(),
			deny:            emptyRole.Spec.Deny,
			wantErrContains: "allow fields not supported by this update: app_labels",
		},
		{
			name: "invalid field",
			allow: func() types.RoleConditions {
				allow := validAllow
				allow.NodeLabels = types.Labels{"offending": {"field"}}
				return allow
			}(),
			deny:            emptyRole.Spec.Deny,
			wantErrContains: "allow fields not supported by this update: node_labels",
		},
		{
			name:            "invalid deny",
			allow:           emptyRole.Spec.Allow,
			deny:            validAllow,
			wantErrContains: "deny fields not supported by this update",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAWSICRoleSpec("some-role-name", emptyRole, tt.allow, tt.deny)
			if tt.wantErrContains != "" {
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), tt.wantErrContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateQueriedRole(t *testing.T) {
	roleName := "some-role-name"
	emptyRole, err := getEmptyRoleV6(roleName, types.V8)
	require.NoError(t, err)

	validAWSICAllow := func() types.RoleConditions {
		allow := emptyRole.Spec.Allow
		allow.AppLabels = awsIcAppLabel
		allow.AccountAssignments = []types.IdentityCenterAccountAssignment{
			{Account: "1234", PermissionSet: "arn:aws:sso:::permissionSet/test"},
		}
		return allow
	}()

	validStandardAllow := func() types.RoleConditions {
		allow := emptyRole.Spec.Allow
		allow.NodeLabels = types.Labels{"env": {"testing"}}
		return allow
	}()

	tests := []struct {
		name                     string
		presetPrefix             string
		roleVersion              string
		allow                    types.RoleConditions
		wantErrContains          string
		wantUnmanagedAllowFields []string
	}{
		{
			name:         "valid standard validator",
			presetPrefix: standardRolePrefixName,
			allow:        validStandardAllow,
		},
		{
			name:                     "valid standard validator fails when given awsic spec",
			presetPrefix:             standardRolePrefixName,
			allow:                    validAWSICAllow,
			wantUnmanagedAllowFields: []string{"account_assignments"},
		},
		{
			name:         "valid awsic validator",
			presetPrefix: awsicRolePrefixName,
			allow:        validAWSICAllow,
		},
		{
			name:                     "valid awsic validator fails when given standard spec",
			presetPrefix:             awsicRolePrefixName,
			allow:                    validStandardAllow,
			wantUnmanagedAllowFields: []string{"node_labels", "app_labels"},
		},
		{
			name:         "do nothing if prefix is not handled",
			presetPrefix: "some-unknown-preset-prefix",
			allow:        types.RoleConditions{ClusterLabels: types.Labels{"env": {"testing"}}},
		},
		{
			name:            "invalid role version",
			presetPrefix:    standardRolePrefixName,
			roleVersion:     types.V7,
			wantErrContains: "unsupported version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roleVersion := types.V8
			if tt.roleVersion != "" {
				roleVersion = tt.roleVersion
			}
			deny := emptyRole.Spec.Deny
			err := validateQueriedRole(tt.presetPrefix, roleName, roleVersion, tt.allow, deny)
			switch {
			case len(tt.wantUnmanagedAllowFields) > 0:
				require.True(t, trace.IsBadParameter(err))
				for _, field := range tt.wantUnmanagedAllowFields {
					require.Contains(t, err.Error(), field)
				}
			case tt.wantErrContains != "":
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), tt.wantErrContains)
			default:
				require.NoError(t, err)
			}
		})
	}
}

func TestRejectUnknownRoles(t *testing.T) {
	tests := []struct {
		name               string
		updateAccessList   func(al *accesslist.AccessList)
		wantErrContains    string
		wantErrContainsAll []string
	}{
		{
			name: "valid empty request",
		},
		{
			name: "valid empty grant fields",
			updateAccessList: func(al *accesslist.AccessList) {
				al.Spec.Grants.Traits = trait.Traits{}
				al.Spec.Grants.ScopedRoles = []accesslist.ScopedRoleGrant{}
				al.Spec.OwnerGrants.Traits = trait.Traits{}
				al.Spec.OwnerGrants.ScopedRoles = []accesslist.ScopedRoleGrant{}
			},
		},
		{
			name: "unknown role in member grant",
			updateAccessList: func(al *accesslist.AccessList) {
				al.Spec.Grants.Roles = append(al.Spec.Grants.Roles, "custom-role", "custom-role2")
			},
			wantErrContains: "grants roles this command doesn't update: custom-role, custom-role2",
		},
		{
			name: "unknown role in owner grant",
			updateAccessList: func(al *accesslist.AccessList) {
				al.Spec.OwnerGrants.Roles = append(al.Spec.OwnerGrants.Roles, "custom-role", "custom-role2")
			},
			wantErrContains: "grants roles this command doesn't update: custom-role, custom-role2",
		},
		{
			name: "invalid member trait field defined",
			updateAccessList: func(al *accesslist.AccessList) {
				al.Spec.Grants.Traits = trait.Traits{"offending": []string{"field"}}
			},
			wantErrContains: "grant fields not supported by this update: traits",
		},
		{
			name: "invalid owner trait field defined",
			updateAccessList: func(al *accesslist.AccessList) {
				al.Spec.OwnerGrants.Traits = trait.Traits{"offending": []string{"field"}}
			},
			wantErrContains: "grant fields not supported by this update: traits",
		},
		{
			name: "invalid member scoped role field defined",
			updateAccessList: func(al *accesslist.AccessList) {
				al.Spec.Grants.ScopedRoles = []accesslist.ScopedRoleGrant{{Role: "role", Scope: "/"}}
			},
			wantErrContains: "grant fields not supported by this update: scoped_roles",
		},
		{
			name: "invalid owner scoped role field defined",
			updateAccessList: func(al *accesslist.AccessList) {
				al.Spec.OwnerGrants.ScopedRoles = []accesslist.ScopedRoleGrant{{Role: "role", Scope: "/"}}
			},
			wantErrContains: "grant fields not supported by this update: scoped_roles",
		},
		{
			name: "multiple invalid fields",
			updateAccessList: func(al *accesslist.AccessList) {
				al.Spec.Grants.Traits = trait.Traits{"offending": []string{"field"}}
				al.Spec.Grants.ScopedRoles = []accesslist.ScopedRoleGrant{{Role: "role", Scope: "/"}}
			},
			wantErrContainsAll: []string{"traits", "scoped_roles"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newAl := newPresetAccessListLongTerm("some-al-name")
			if tt.updateAccessList != nil {
				tt.updateAccessList(newAl)
			}

			err := rejectUnknownGrants(newAl)
			switch {
			case len(tt.wantErrContainsAll) > 0:
				require.True(t, trace.IsBadParameter(err))
				for _, want := range tt.wantErrContainsAll {
					require.Contains(t, err.Error(), want)
				}
			case tt.wantErrContains != "":
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), tt.wantErrContains)
			default:
				require.NoError(t, err)
			}
		})
	}
}

func TestSupportsRoleVersion(t *testing.T) {
	tests := []struct {
		name            string
		gotVersion      string
		wantErrContains string
	}{
		{
			name:       "minimum supported version",
			gotVersion: "v8",
		},
		{
			name:       "default version",
			gotVersion: types.DefaultRoleVersion,
		},
		{
			name:            "below minimum version",
			gotVersion:      "v7",
			wantErrContains: "unsupported version",
		},
		{
			name:            "above default version",
			gotVersion:      "v9999999999999999999",
			wantErrContains: "Upgrade your tctl binary",
		},
		{
			name:            "malformed version",
			gotVersion:      "not-a-version",
			wantErrContains: "unsupported version",
		},
		{
			name:            "empty version",
			gotVersion:      "",
			wantErrContains: "unsupported version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := supportsRoleVersion(tt.gotVersion, "some-role-name")
			if tt.wantErrContains != "" {
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), tt.wantErrContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func newPresetAccessListLongTerm(name string) *accesslist.AccessList {
	reviewerRole := accesslist.RoleName(preset.RoleReviewerPrefix, name)
	requesterRole := accesslist.RoleName(preset.RoleRequesterPrefix, name)
	standardRole := accesslist.RoleName(standardRolePrefixName, name)
	awsicRole := accesslist.RoleName(awsicRolePrefixName, name)

	al := &accesslist.AccessList{}
	al.Metadata.Name = name
	al.Metadata.Labels = map[string]string{
		accesslist.AccessListPresetLabel:      string(accesslist.LongTermPresetType),
		accesslist.AccessListPresetRolesLabel: strings.Join([]string{reviewerRole, requesterRole, standardRole, awsicRole}, ","),
	}
	al.Spec.Grants = accesslist.Grants{Roles: []string{standardRole, awsicRole}}
	al.Spec.OwnerGrants = accesslist.Grants{Roles: []string{reviewerRole}}
	return al
}
