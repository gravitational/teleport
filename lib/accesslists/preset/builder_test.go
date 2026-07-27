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

package preset_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/accesslists/preset"
)

func TestPresetAccessListRolesBuilder(t *testing.T) {
	type check func(t *testing.T, result *preset.BuildResult)

	accessListName := "test-access-list"
	appRoleName := "app-access-acl-preset-test-access-list"
	dbRoleName := "db-access-acl-preset-test-access-list"
	reviewerRoleName := "reviewer-acl-preset-test-access-list"
	requesterRoleName := "requester-acl-preset-test-access-list"

	appRole, err := types.NewRole("app-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: types.Labels{"env": []string{"production"}},
		},
	})
	require.NoError(t, err)

	dbRole, err := types.NewRole("db-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			DatabaseLabels: types.Labels{"env": []string{"production"}},
		},
	})
	require.NoError(t, err)

	al, err := accesslist.NewAccessList(
		header.Metadata{Name: accessListName},
		accesslist.Spec{Title: "Test Access List"},
	)
	require.NoError(t, err)

	checkAccessListMetadata := func(presetTypeLabel string) check {
		return func(t *testing.T, result *preset.BuildResult) {
			require.Equal(t, accessListName, result.AccessList.GetName())
			require.Equal(t, presetTypeLabel, result.AccessList.GetMetadata().Labels[accesslist.AccessListPresetLabel])
			require.Equal(t, "Test Access List", result.AccessList.Spec.Title)
		}
	}

	checkAccessListGrants := func(memberRoles, ownerRoles []string) check {
		return func(t *testing.T, result *preset.BuildResult) {
			require.ElementsMatch(t, memberRoles, result.AccessList.Spec.Grants.Roles)
			require.Equal(t, ownerRoles, result.AccessList.Spec.OwnerGrants.Roles)
		}
	}

	checkAccessRole := func(index int, name string, appLabels, dbLabels types.Labels) check {
		return func(t *testing.T, result *preset.BuildResult) {
			require.Greater(t, len(result.AccessRoles), index, "role index out of bounds")

			role := result.AccessRoles[index]
			require.Equal(t, name, role.GetName())
			require.Equal(t, accessListName, role.GetMetadata().Labels[accesslist.AccessListPresetLabel])
			require.Equal(t, preset.RoleDesc, role.GetMetadata().Description)

			// Check app labels if specified
			if appLabels != nil {
				require.Equal(t, appLabels, role.GetAppLabels(types.Allow))
			}

			// Check database labels if specified
			if dbLabels != nil {
				require.Equal(t, dbLabels, role.GetDatabaseLabels(types.Allow))
			}
		}
	}

	checkReviewerRoleCanReviewRoles := func(roles []string) check {
		return func(t *testing.T, result *preset.BuildResult) {
			require.Equal(t, reviewerRoleName, result.ReviewerRole.GetName())
			require.Equal(t, accessListName, result.ReviewerRole.GetMetadata().Labels[accesslist.AccessListPresetLabel])
			require.Equal(t, preset.RoleDesc, result.ReviewerRole.GetMetadata().Description)

			reviewCond := result.ReviewerRole.GetAccessReviewConditions(types.Allow)
			require.ElementsMatch(t, roles, reviewCond.Roles)
			require.ElementsMatch(t, roles, reviewCond.PreviewAsRoles)
		}
	}

	checkRequesterRoleCanSearchAsRoles := func(roles []string) check {
		return func(t *testing.T, result *preset.BuildResult) {
			require.Equal(t, requesterRoleName, result.RequesterRole.GetName())
			require.Equal(t, accessListName, result.RequesterRole.GetMetadata().Labels[accesslist.AccessListPresetLabel])
			require.Equal(t, preset.RoleDesc, result.RequesterRole.GetMetadata().Description)

			requestCond := result.RequesterRole.GetAccessRequestConditions(types.Allow)
			require.ElementsMatch(t, roles, requestCond.SearchAsRoles)
		}
	}

	checkNoRolesToDelete := func() check {
		return func(t *testing.T, result *preset.BuildResult) {
			require.Nil(t, result.RolesToBeDeleted)
		}
	}

	tests := []struct {
		name       string
		presetType preset.PresetType
		checks     []check
	}{
		{
			name:       "LongTerm",
			presetType: preset.LongTermPresetType,
			checks: []check{
				checkAccessListMetadata("long-term"),
				checkAccessListGrants(
					[]string{appRoleName, dbRoleName}, // Members get access roles directly
					[]string{reviewerRoleName},        // Owners get reviewer role
				),
				checkAccessRole(0, appRoleName, types.Labels{"env": []string{"production"}}, nil),
				checkAccessRole(1, dbRoleName, nil, types.Labels{"env": []string{"production"}}),
				checkReviewerRoleCanReviewRoles([]string{appRoleName, dbRoleName}),
				checkRequesterRoleCanSearchAsRoles([]string{appRoleName, dbRoleName}),
				checkNoRolesToDelete(),
			},
		},
		{
			name:       "ShortTerm",
			presetType: preset.ShortTermPresetType,
			checks: []check{
				checkAccessListMetadata("short-term"),
				checkAccessListGrants(
					[]string{requesterRoleName}, // Members get requester role (to request access)
					[]string{reviewerRoleName},  // Owners get reviewer role
				),
				checkAccessRole(0, appRoleName, types.Labels{"env": []string{"production"}}, nil),
				checkAccessRole(1, dbRoleName, nil, types.Labels{"env": []string{"production"}}),
				checkReviewerRoleCanReviewRoles([]string{appRoleName, dbRoleName}),
				checkRequesterRoleCanSearchAsRoles([]string{appRoleName, dbRoleName}),
				checkNoRolesToDelete(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder, err := preset.NewPresetAccessListRolesBuilder(preset.AccessListRolesBuilderConfig{
				PresetName:     accessListName,
				PresetType:     tt.presetType,
				AccessRoles:    []types.Role{appRole, dbRole},
				AccessListSpec: al,
			})
			require.NoError(t, err)

			result, err := builder.Build()
			require.NoError(t, err)

			for _, check := range tt.checks {
				check(t, result)
			}
		})
	}

	t.Run("nil builtRoles leaves grants empty", func(t *testing.T) {
		builder, err := preset.NewPresetAccessListRolesBuilder(preset.AccessListRolesBuilderConfig{
			PresetName:     accessListName,
			PresetType:     preset.LongTermPresetType,
			AccessRoles:    []types.Role{appRole},
			AccessListSpec: al,
		})
		require.NoError(t, err)

		result, err := builder.BuildAccessList(nil)
		require.NoError(t, err)

		require.Empty(t, result.AccessList.Spec.Grants.Roles)
		require.Empty(t, result.AccessList.Spec.OwnerGrants.Roles)
		require.NotContains(t, result.AccessList.GetStaticLabels(), accesslist.AccessListPresetRolesLabel)
	})

	t.Run("nil access list spec returns error", func(t *testing.T) {
		_, err := preset.NewPresetAccessListRolesBuilder(preset.AccessListRolesBuilderConfig{
			PresetName: accessListName,
			PresetType: preset.LongTermPresetType,
		})
		require.Error(t, err)
	})

	t.Run("preset name mismatch", func(t *testing.T) {
		otherAccessList, err := accesslist.NewAccessList(
			header.Metadata{Name: "different-name"},
			accesslist.Spec{Title: "Other Access List"},
		)
		require.NoError(t, err)
		_, err = preset.NewPresetAccessListRolesBuilder(preset.AccessListRolesBuilderConfig{
			PresetName:     accessListName,
			PresetType:     preset.LongTermPresetType,
			AccessListSpec: otherAccessList,
		})
		require.ErrorContains(t, err, "access list name is invalid")
	})
}

func TestPresetAccessListRolesBuilder_RolesToBeDeleted(t *testing.T) {
	accessListName := "test-access-list"

	existingAL, err := accesslist.NewAccessList(
		header.Metadata{
			Name: accessListName,
		},
		accesslist.Spec{
			Title:       "Test Access List",
			Description: "Test description",
			Grants: accesslist.Grants{
				Roles: []string{
					"old-role-1-acl-preset-test-access-list",
					"old-role-2-acl-preset-test-access-list",
				},
			},
		},
	)
	require.NoError(t, err)

	newRole, err := types.NewRole("new-role", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: types.Labels{"env": []string{"production"}},
		},
	})
	require.NoError(t, err)

	builder, err := preset.NewPresetAccessListRolesBuilder(preset.AccessListRolesBuilderConfig{
		PresetName:     accessListName,
		PresetType:     preset.LongTermPresetType,
		AccessRoles:    []types.Role{newRole},
		AccessListSpec: existingAL,
	})
	require.NoError(t, err)

	result, err := builder.Build()
	require.NoError(t, err)

	require.Len(t, result.AccessRoles, 1)
	require.Equal(t, "new-role-acl-preset-test-access-list", result.AccessRoles[0].GetName())
}

func TestPresetAccessListRolesBuilderForTerraform(t *testing.T) {
	accessListName := "test-access-list"
	accessRole1Name := "accessRole1-acl-preset-test-access-list"
	accessRole2Name := "accessRole2-acl-preset-test-access-list"
	reviewerRoleName := "reviewer-acl-preset-test-access-list"
	requesterRoleName := "requester-acl-preset-test-access-list"

	rolesLabelValue := strings.Join([]string{reviewerRoleName, requesterRoleName, accessRole1Name, accessRole2Name}, ",")

	accessList, err := accesslist.NewAccessList(
		header.Metadata{Name: accessListName},
		accesslist.Spec{Title: "Test Access List"},
	)
	require.NoError(t, err)

	accessRole1, err := types.NewRole("accessRole1", types.RoleSpecV6{})
	require.NoError(t, err)

	accessRole2, err := types.NewRole("accessRole2", types.RoleSpecV6{})
	require.NoError(t, err)

	checkCommonLabelsAndRoles := func(result *preset.BuildResult) {
		require.Equal(t, []string{reviewerRoleName}, result.AccessList.Spec.OwnerGrants.Roles)

		staticLabels := result.AccessList.GetStaticLabels()
		require.Len(t, result.AccessRoles, 2)
		require.Equal(t, types.IACToolTerraform, staticLabels[types.IACToolLabel])
		require.Equal(t, rolesLabelValue, staticLabels[accesslist.AccessListPresetRolesLabel])

		require.Len(t, result.GetAllRoles(), 4) // 2 access roles + reviewer + requester
		for _, role := range result.GetAllRoles() {
			labels := role.GetStaticLabels()
			require.Equal(t, types.IACToolTerraform, labels[types.IACToolLabel], "role %s missing terraform label", role.GetName())
			require.Equal(t, accessListName, labels[accesslist.AccessListPresetLabel], "role %s missing access list preset label", role.GetName())
			require.Empty(t, role.GetMetadata().Description, "role %s should have no description", role.GetName())
		}
	}

	tests := []struct {
		name        string
		presetType  preset.PresetType
		accessList  *accesslist.AccessList
		accessRoles []types.Role
		validate    func(t *testing.T, result *preset.BuildResult)
	}{
		{
			name:        "long term build",
			presetType:  preset.LongTermPresetType,
			accessList:  accessList,
			accessRoles: []types.Role{accessRole1, accessRole2},
			validate: func(t *testing.T, result *preset.BuildResult) {
				checkCommonLabelsAndRoles(result)
				require.Equal(t, string(preset.LongTermPresetType), result.AccessList.GetMetadata().Labels[accesslist.AccessListPresetLabel])
				require.ElementsMatch(t, []string{accessRole1Name, accessRole2Name}, result.AccessList.Spec.Grants.Roles)
			},
		},
		{
			name:        "short term build",
			presetType:  preset.ShortTermPresetType,
			accessList:  accessList,
			accessRoles: []types.Role{accessRole1, accessRole2},
			validate: func(t *testing.T, result *preset.BuildResult) {
				checkCommonLabelsAndRoles(result)
				require.Equal(t, string(preset.ShortTermPresetType), result.AccessList.GetMetadata().Labels[accesslist.AccessListPresetLabel])
				require.ElementsMatch(t, []string{requesterRoleName}, result.AccessList.Spec.Grants.Roles)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			builder, err := preset.NewPresetAccessListRolesBuilderForTerraform(preset.AccessListRolesBuilderConfig{
				PresetName:     accessListName,
				PresetType:     tc.presetType,
				AccessRoles:    tc.accessRoles,
				AccessListSpec: tc.accessList,
			})
			require.NoError(t, err)

			result, err := builder.Build()
			require.NoError(t, err)

			tc.validate(t, result)
		})
	}

	t.Run("nil builtRoles leaves grants empty", func(t *testing.T) {
		builder, err := preset.NewPresetAccessListRolesBuilderForTerraform(preset.AccessListRolesBuilderConfig{
			PresetName:     accessListName,
			PresetType:     preset.ShortTermPresetType,
			AccessListSpec: accessList,
		})
		require.NoError(t, err)

		builtRoles, err := builder.BuildRoles()
		require.NoError(t, err)
		require.Empty(t, builtRoles.AccessRoles)

		builtAl, err := builder.BuildAccessList(nil)
		require.NoError(t, err)
		require.NotNil(t, builtAl.AccessList)
		require.Empty(t, builtAl.AccessList.Spec.Grants.Roles)
		require.Empty(t, builtAl.AccessList.Spec.OwnerGrants.Roles)
		require.NotContains(t, builtAl.AccessList.GetStaticLabels(), accesslist.AccessListPresetRolesLabel)
	})

	t.Run("invalid preset type", func(t *testing.T) {
		_, err := preset.NewPresetAccessListRolesBuilderForTerraform(preset.AccessListRolesBuilderConfig{
			PresetName:     accessListName,
			PresetType:     "unknown",
			AccessListSpec: accessList,
		})
		require.ErrorContains(t, err, "preset type is required")
	})

	t.Run("preset name mismatch", func(t *testing.T) {
		otherAccessList, err := accesslist.NewAccessList(
			header.Metadata{Name: "different-name"},
			accesslist.Spec{Title: "Other Access List"},
		)
		require.NoError(t, err)
		_, err = preset.NewPresetAccessListRolesBuilderForTerraform(preset.AccessListRolesBuilderConfig{
			PresetName:     accessListName,
			PresetType:     preset.LongTermPresetType,
			AccessListSpec: otherAccessList,
		})
		require.ErrorContains(t, err, "access list name is invalid")
	})
}

func TestPresetAccessListRolesBuilder_Added(t *testing.T) {
	accessListName := "test-access-list"

	al, err := accesslist.NewAccessList(
		header.Metadata{Name: accessListName},
		accesslist.Spec{Title: "Test Access List"},
	)
	require.NoError(t, err)

	devRole, err := types.NewRole("dev", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: types.Labels{"env": []string{"dev"}},
		},
	})
	require.NoError(t, err)

	prodRole, err := types.NewRole("prod", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: types.Labels{"env": []string{"prod"}},
		},
	})
	require.NoError(t, err)

	builder, err := preset.NewPresetAccessListRolesBuilder(preset.AccessListRolesBuilderConfig{
		PresetName:     accessListName,
		PresetType:     preset.LongTermPresetType,
		AccessRoles:    []types.Role{devRole},
		AccessListSpec: al,
	})
	require.NoError(t, err)

	result, err := builder.Build()
	require.NoError(t, err)

	require.NotNil(t, result.AccessList)
	require.Empty(t, result.RolesToBeDeleted)
	require.Len(t, result.AccessRoles, 1)
	require.Equal(t, result.AccessRoles[0].GetName(), preset.RoleName(devRole.GetName(), accessListName))

	builder, err = preset.NewPresetAccessListRolesBuilder(preset.AccessListRolesBuilderConfig{
		PresetName:     accessListName,
		PresetType:     preset.ShortTermPresetType,
		AccessRoles:    []types.Role{result.AccessRoles[0], prodRole},
		AccessListSpec: result.AccessList,
	})
	require.NoError(t, err)

	result, err = builder.Build()
	require.NoError(t, err)

	require.NotNil(t, result.AccessList)
	require.Empty(t, result.RolesToBeDeleted)
	require.Len(t, result.AccessRoles, 2)
	require.Equal(t, result.AccessRoles[0].GetName(), preset.RoleName(devRole.GetName(), accessListName))
	require.Equal(t, result.AccessRoles[1].GetName(), preset.RoleName(prodRole.GetName(), accessListName))

	builder, err = preset.NewPresetAccessListRolesBuilder(preset.AccessListRolesBuilderConfig{
		PresetName:     accessListName,
		PresetType:     preset.ShortTermPresetType,
		AccessRoles:    []types.Role{result.AccessRoles[0]},
		AccessListSpec: result.AccessList,
	})
	require.NoError(t, err)

	result, err = builder.Build()
	require.NoError(t, err)

	require.NotNil(t, result.AccessList)
	require.Len(t, result.RolesToBeDeleted, 1)
	require.Equal(t, result.AccessRoles[0].GetName(), preset.RoleName(devRole.GetName(), accessListName))
}

func TestPresetAccessListRolesBuilder_PreservesExistingRoleLabels(t *testing.T) {
	accessListName := "test-access-list"

	al, err := accesslist.NewAccessList(
		header.Metadata{Name: accessListName},
		accesslist.Spec{Title: "Test Access List"},
	)
	require.NoError(t, err)

	existingRole, err := types.NewRole(
		preset.RoleName("app-access", accessListName),
		types.RoleSpecV6{
			Allow: types.RoleConditions{
				AppLabels: types.Labels{"env": []string{"production"}},
			},
		},
	)
	require.NoError(t, err)
	existingRole.SetStaticLabels(map[string]string{
		accesslist.AccessListPresetLabel: accessListName,
		"owner":                          "team-foo",
		"env":                            "dev",
	})

	t.Run("non-IAC update preserves all existing labels", func(t *testing.T) {
		builder, err := preset.NewPresetAccessListRolesBuilder(preset.AccessListRolesBuilderConfig{
			PresetName:     accessListName,
			PresetType:     preset.LongTermPresetType,
			AccessRoles:    []types.Role{existingRole.Clone()},
			AccessListSpec: al,
		})
		require.NoError(t, err)

		result, err := builder.Build()
		require.NoError(t, err)

		require.Len(t, result.AccessRoles, 1)
		labels := result.AccessRoles[0].GetStaticLabels()
		require.Equal(t, accessListName, labels[accesslist.AccessListPresetLabel])
		require.Equal(t, "team-foo", labels["owner"])
		require.Equal(t, "dev", labels["env"])
	})

	t.Run("IAC update preserves unrelated labels and adds IAC label", func(t *testing.T) {
		builder, err := preset.NewPresetAccessListRolesBuilderForTerraform(preset.AccessListRolesBuilderConfig{
			PresetName:     accessListName,
			PresetType:     preset.LongTermPresetType,
			AccessRoles:    []types.Role{existingRole.Clone()},
			AccessListSpec: al,
		})
		require.NoError(t, err)

		result, err := builder.Build()
		require.NoError(t, err)

		require.Len(t, result.AccessRoles, 1)
		labels := result.AccessRoles[0].GetStaticLabels()
		require.Equal(t, accessListName, labels[accesslist.AccessListPresetLabel])
		require.Equal(t, types.IACToolTerraform, labels[types.IACToolLabel])
		require.Equal(t, "team-foo", labels["owner"])
		require.Equal(t, "dev", labels["env"])
	})
}

func TestNewPresetAccessListRolesBuilder_Validation(t *testing.T) {
	accessListName := "test-access-list"

	al, err := accesslist.NewAccessList(
		header.Metadata{Name: accessListName},
		accesslist.Spec{Title: "Test Access List"},
	)
	require.NoError(t, err)

	tests := []struct {
		name    string
		cfg     preset.AccessListRolesBuilderConfig
		wantErr string
	}{
		{
			name: "nil access list spec",
			cfg: preset.AccessListRolesBuilderConfig{
				PresetName:     accessListName,
				PresetType:     preset.LongTermPresetType,
				AccessListSpec: nil,
			},
			wantErr: "access list is required",
		},
		{
			name: "invalid preset type",
			cfg: preset.AccessListRolesBuilderConfig{
				PresetName:     accessListName,
				PresetType:     "unknown",
				AccessListSpec: al,
			},
			wantErr: "preset type is required",
		},
		{
			name: "preset name mismatch",
			cfg: preset.AccessListRolesBuilderConfig{
				PresetName:     "different-name",
				PresetType:     preset.LongTermPresetType,
				AccessListSpec: al,
			},
			wantErr: "access list name is invalid",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := preset.NewPresetAccessListRolesBuilder(tc.cfg)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}
