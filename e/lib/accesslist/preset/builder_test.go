package preset_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/e/lib/accesslist/preset"
)

func TestPresetAccessListRolesBuilder(t *testing.T) {
	type check func(t *testing.T, result *preset.BuildResult)

	accessListName := "test-access-list"
	appRoleName := "app-access-acl-preset-test-access-list"
	dbRoleName := "db-access-acl-preset-test-access-list"
	reviewerRoleName := "reviewer-acl-preset-test-access-list"
	requesterRoleName := "requester-acl-preset-test-access-list"

	checkAccessListMetadata := func(presetTypeLabel string) check {
		return func(t *testing.T, result *preset.BuildResult) {
			require.Equal(t, accessListName, result.AccessList.GetName())
			require.Equal(t, presetTypeLabel, result.AccessList.GetMetadata().Labels[preset.TeleportAccessListPreset])
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
			require.Equal(t, accessListName, role.GetMetadata().Labels[preset.TeleportAccessListPreset])
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
			require.Equal(t, accessListName, result.ReviewerRole.GetMetadata().Labels[preset.TeleportAccessListPreset])
			require.Equal(t, preset.RoleDesc, result.ReviewerRole.GetMetadata().Description)

			reviewCond := result.ReviewerRole.GetAccessReviewConditions(types.Allow)
			require.ElementsMatch(t, roles, reviewCond.Roles)
			require.ElementsMatch(t, roles, reviewCond.PreviewAsRoles)
		}
	}

	checkRequesterRoleCanSearchAsRoles := func(roles []string) check {
		return func(t *testing.T, result *preset.BuildResult) {
			require.Equal(t, requesterRoleName, result.RequesterRole.GetName())
			require.Equal(t, accessListName, result.RequesterRole.GetMetadata().Labels[preset.TeleportAccessListPreset])
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

			builder, err := preset.NewPresetAccessListRolesBuilder(preset.AccessListRolesBuilderConfig{
				PresetName:     accessListName,
				PresetType:     tt.presetType,
				AccessRoles:    []types.Role{appRole, dbRole},
				AccessListSpec: *al,
			})
			require.NoError(t, err)

			result, err := builder.Build()
			require.NoError(t, err)

			for _, check := range tt.checks {
				check(t, result)
			}
		})
	}
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
		AccessListSpec: *existingAL,
	})
	require.NoError(t, err)

	result, err := builder.Build()
	require.NoError(t, err)

	require.Len(t, result.AccessRoles, 1)
	require.Equal(t, "new-role-acl-preset-test-access-list", result.AccessRoles[0].GetName())
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
		AccessListSpec: *al,
	})
	require.NoError(t, err)

	result, err := builder.Build()
	require.NoError(t, err)

	require.Empty(t, result.RolesToBeDeleted)
	require.Len(t, result.AccessRoles, 1)
	require.Equal(t, result.AccessRoles[0].GetName(), preset.RoleName(devRole.GetName(), accessListName))

	builder, err = preset.NewPresetAccessListRolesBuilder(preset.AccessListRolesBuilderConfig{
		PresetName:     accessListName,
		PresetType:     preset.ShortTermPresetType,
		AccessRoles:    []types.Role{result.AccessRoles[0], prodRole},
		AccessListSpec: *result.AccessList,
	})
	require.NoError(t, err)

	result, err = builder.Build()
	require.NoError(t, err)

	require.Empty(t, result.RolesToBeDeleted)
	require.Len(t, result.AccessRoles, 2)
	require.Equal(t, result.AccessRoles[0].GetName(), preset.RoleName(devRole.GetName(), accessListName))
	require.Equal(t, result.AccessRoles[1].GetName(), preset.RoleName(prodRole.GetName(), accessListName))

	builder, err = preset.NewPresetAccessListRolesBuilder(preset.AccessListRolesBuilderConfig{
		PresetName:     accessListName,
		PresetType:     preset.ShortTermPresetType,
		AccessRoles:    []types.Role{result.AccessRoles[0]},
		AccessListSpec: *result.AccessList,
	})
	require.NoError(t, err)

	result, err = builder.Build()
	require.NoError(t, err)

	require.Len(t, result.RolesToBeDeleted, 1)
	require.Equal(t, result.AccessRoles[0].GetName(), preset.RoleName(devRole.GetName(), accessListName))
}
