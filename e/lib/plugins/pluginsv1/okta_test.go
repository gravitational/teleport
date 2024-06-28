package pluginsv1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
)

func TestService_CleanupOkta(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
	})

	suite := createSuite(t)
	suite.setRules([]types.Rule{
		{Resources: []string{types.KindPlugin}, Verbs: services.RW()},
	})

	ctx := context.Background()

	expectedNeedsCleanup := func(active bool, resourcesToCleanup ...*types.ResourceID) {
		resp, err := suite.svc.NeedsCleanup(ctx, &pluginspb.NeedsCleanupRequest{
			Type: types.PluginTypeOkta,
		})
		require.NoError(t, err)
		require.Equal(t, len(resourcesToCleanup) > 0, resp.NeedsCleanup)
		require.Equal(t, resourcesToCleanup, resp.ResourcesToCleanup)
		require.Equal(t, active, resp.PluginActive)
	}

	// Create a bunch of Okta sourced resources.
	oktaAssignments := []types.OktaAssignment{
		newOktaAssignment(t, "assignment1"),
		newOktaAssignment(t, "assignment2"),
		newOktaAssignment(t, "assignment3"),
	}
	accessListsToCleanup := []*accesslist.AccessList{
		newAccessList(t, "al1-cleanup", types.OriginOkta),
		newAccessList(t, "al2-cleanup", types.OriginOkta),
		newAccessList(t, "al3-cleanup", types.OriginOkta),
	}
	accessLists := []*accesslist.AccessList{
		newAccessList(t, "al4", ""),
		newAccessList(t, "al5", ""),
		newAccessList(t, "al6", ""),
	}
	rolesToCleanup := []types.Role{
		newRole(t, "r1-cleanup", types.OriginOkta),
		newRole(t, "r2-cleanup", types.OriginOkta),
		newRole(t, "r3-cleanup", types.OriginOkta),
	}
	oktaAccessRole := services.NewSystemOktaAccessRole()
	oktaRequesterRole := services.NewSystemOktaRequesterRole()
	roles := []types.Role{
		oktaAccessRole,
		oktaRequesterRole, // This is a special role that shouldn't be deleted, but it should be modified.
		newRole(t, "r4", ""),
		newRole(t, "r5", ""),
		newRole(t, "r6", ""),
	}

	// Roles need to be inserted first.
	upsertRoles(t, ctx, suite.svc.authServer, roles...)

	// Make sure Okta assignments can cause a needs cleanup.
	expectedNeedsCleanup(false)
	upsertOktaAssignments(t, ctx, suite.svc.authServer, oktaAssignments)
	expectedNeedsCleanup(false,
		&types.ResourceID{Kind: types.KindOktaAssignment, Name: "assignment1"},
		&types.ResourceID{Kind: types.KindOktaAssignment, Name: "assignment2"},
		&types.ResourceID{Kind: types.KindOktaAssignment, Name: "assignment3"},
	)
	deleteOktaAssignments(t, ctx, suite.svc.authServer, oktaAssignments)

	// Make sure access lists can cause a needs cleanup.
	upsertAccessLists(t, ctx, suite.svc.authServer, accessLists)
	expectedNeedsCleanup(false)
	upsertAccessLists(t, ctx, suite.svc.authServer, accessListsToCleanup)
	expectedNeedsCleanup(false,
		&types.ResourceID{Kind: types.KindAccessList, Name: "al1-cleanup"},
		&types.ResourceID{Kind: types.KindAccessList, Name: "al2-cleanup"},
		&types.ResourceID{Kind: types.KindAccessList, Name: "al3-cleanup"},
	)
	deleteAccessLists(t, ctx, suite.svc.authServer, accessListsToCleanup)

	// Make sure roles can cause a needs cleanup.
	upsertRoles(t, ctx, suite.svc.authServer, roles...)
	expectedNeedsCleanup(false)
	upsertRoles(t, ctx, suite.svc.authServer, rolesToCleanup...)
	expectedNeedsCleanup(false,
		&types.ResourceID{Kind: types.KindRole, Name: "r1-cleanup"},
		&types.ResourceID{Kind: types.KindRole, Name: "r2-cleanup"},
		&types.ResourceID{Kind: types.KindRole, Name: "r3-cleanup"},
	)
	deleteRoles(t, ctx, suite.svc.authServer, rolesToCleanup)

	// Make sure that the Okta requester role can cause a cleanup.
	oktaRequesterRole.SetSearchAsRoles(types.Allow, []string{"r1-cleanup", "r2-cleanup", "r3-cleanup"})
	upsertRoles(t, ctx, suite.svc.authServer, oktaRequesterRole)

	expectedNeedsCleanup(false,
		&types.ResourceID{Kind: types.KindRole, Name: teleport.SystemOktaRequesterRoleName},
	)

	// Reset the role so that it's back to normal.
	upsertRoles(t, ctx, suite.svc.authServer, services.NewSystemOktaRequesterRole())

	staticCredentials := &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: "static-creds",
				Labels: map[string]string{
					"label1":                          "value1",
					"label2":                          "value2",
					types.TeleportInternalLabelPrefix: "filtered",
				},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
				APIToken: "some-token",
			},
		},
	}
	oktaPlugin := types.NewPluginV1(
		types.Metadata{Name: "okta-default"},
		types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Okta{
				Okta: &types.PluginOktaSettings{
					OrgUrl: "https://www.okta.com",
				},
			},
		},
		nil)
	_, err := suite.svc.CreatePlugin(ctx, &pluginspb.CreatePluginRequest{
		Plugin:            oktaPlugin,
		StaticCredentials: staticCredentials,
	})
	require.NoError(t, err)

	expectedNeedsCleanup(true)

	// Put all of the resources that need a cleanup back in.
	upsertOktaAssignments(t, ctx, suite.svc.authServer, oktaAssignments)
	upsertAccessLists(t, ctx, suite.svc.authServer, accessListsToCleanup)
	upsertRoles(t, ctx, suite.svc.authServer, rolesToCleanup...)
	oktaRequesterRole.SetSearchAsRoles(types.Allow, []string{"r1-cleanup", "r2-cleanup", "r3-cleanup"})
	upsertRoles(t, ctx, suite.svc.authServer, oktaRequesterRole)

	// The plugin is active.
	expectedNeedsCleanup(true,
		&types.ResourceID{Kind: types.KindOktaAssignment, Name: "assignment1"},
		&types.ResourceID{Kind: types.KindOktaAssignment, Name: "assignment2"},
		&types.ResourceID{Kind: types.KindOktaAssignment, Name: "assignment3"},
		&types.ResourceID{Kind: types.KindAccessList, Name: "al1-cleanup"},
		&types.ResourceID{Kind: types.KindAccessList, Name: "al2-cleanup"},
		&types.ResourceID{Kind: types.KindAccessList, Name: "al3-cleanup"},
		&types.ResourceID{Kind: types.KindRole, Name: "r1-cleanup"},
		&types.ResourceID{Kind: types.KindRole, Name: "r2-cleanup"},
		&types.ResourceID{Kind: types.KindRole, Name: "r3-cleanup"},
		&types.ResourceID{Kind: types.KindRole, Name: teleport.SystemOktaRequesterRoleName},
	)

	// Cleanup fails due to an active plugin.
	_, err = suite.svc.Cleanup(ctx, &pluginspb.CleanupRequest{
		Type: types.PluginTypeOkta,
	})
	require.ErrorContains(t, err, "can't cleanup")

	// Delete the plugin, which means the plugin should no longer be active.
	_, err = suite.svc.DeletePlugin(ctx, &pluginspb.DeletePluginRequest{
		Name: oktaPlugin.GetName(),
	})
	require.NoError(t, err)

	expectedNeedsCleanup(false,
		&types.ResourceID{Kind: types.KindOktaAssignment, Name: "assignment1"},
		&types.ResourceID{Kind: types.KindOktaAssignment, Name: "assignment2"},
		&types.ResourceID{Kind: types.KindOktaAssignment, Name: "assignment3"},
		&types.ResourceID{Kind: types.KindAccessList, Name: "al1-cleanup"},
		&types.ResourceID{Kind: types.KindAccessList, Name: "al2-cleanup"},
		&types.ResourceID{Kind: types.KindAccessList, Name: "al3-cleanup"},
		&types.ResourceID{Kind: types.KindRole, Name: "r1-cleanup"},
		&types.ResourceID{Kind: types.KindRole, Name: "r2-cleanup"},
		&types.ResourceID{Kind: types.KindRole, Name: "r3-cleanup"},
		&types.ResourceID{Kind: types.KindRole, Name: teleport.SystemOktaRequesterRoleName},
	)

	// Cleanup should succeed.
	_, err = suite.svc.Cleanup(ctx, &pluginspb.CleanupRequest{
		Type: types.PluginTypeOkta,
	})
	require.NoError(t, err)

	backendOktaAssignments, _, err := suite.svc.authServer.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)
	backendAccessLists, _, err := suite.svc.authServer.ListAccessLists(ctx, 0, "")
	require.NoError(t, err)
	backendRolesResp, err := suite.svc.authServer.ListRoles(ctx, &proto.ListRolesRequest{})
	require.NoError(t, err)
	backendRoles := make([]types.Role, 0, len(backendRolesResp.Roles))
	for _, role := range backendRolesResp.Roles {
		// Ignore the default implicit role.
		if role.GetName() != constants.DefaultImplicitRole {
			backendRoles = append(backendRoles, role)
		}
	}

	// Reset the Okta requester role, while we test for its definition, as it should have been reset as part
	// of this. We'll get from the backend so we get whatever backend defaults are set here and just
	// reset the search as roles bit.
	oktaRequesterRole, err = suite.svc.authServer.GetRole(ctx, teleport.SystemOktaRequesterRoleName)
	require.NoError(t, err)
	roles[1] = oktaRequesterRole
	roles[1].SetSearchAsRoles(types.Allow, services.NewSystemOktaRequesterRole().GetSearchAsRoles(types.Allow))

	require.Empty(t, backendOktaAssignments)
	require.Empty(t, cmp.Diff(accessLists, backendAccessLists, cmpopts.IgnoreFields(header.Metadata{}, "Revision")))
	require.Empty(t, cmp.Diff(roles, backendRoles, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
}

func newOktaAssignment(t *testing.T, name string) types.OktaAssignment {
	t.Helper()

	assignment, err := types.NewOktaAssignment(
		types.Metadata{
			Name: name,
		},
		types.OktaAssignmentSpecV1{
			User: "some-user",
			Targets: []*types.OktaAssignmentTargetV1{
				{
					Type: types.OktaAssignmentTargetV1_APPLICATION,
					Id:   "some-id",
				},
			},
		},
	)
	require.NoError(t, err)

	return assignment
}

func newAccessList(t *testing.T, name, origin string) *accesslist.AccessList {
	t.Helper()

	var labels map[string]string
	if origin != "" {
		labels = map[string]string{}
		labels[types.OriginLabel] = origin
	}

	accessList, err := accesslist.NewAccessList(header.Metadata{
		Name:   name,
		Labels: labels,
	}, accesslist.Spec{
		Title: "some title",
		OwnerGrants: accesslist.Grants{
			Roles: []string{"grant-role"},
		},
		Grants: accesslist.Grants{
			Roles: []string{"role"},
		},
		Owners: []accesslist.Owner{
			{

				Name: "some-owner",
			},
		},
	})
	require.NoError(t, err)

	return accessList
}

func upsertOktaAssignments(t *testing.T, ctx context.Context, ap services.Okta, oktaAssignments []types.OktaAssignment) {
	t.Helper()

	for _, oktaAssignment := range oktaAssignments {
		_, err := ap.CreateOktaAssignment(ctx, oktaAssignment)
		require.NoError(t, err)
	}
}

func deleteOktaAssignments(t *testing.T, ctx context.Context, ap services.Okta, oktaAssignments []types.OktaAssignment) {
	t.Helper()

	for _, oktaAssignment := range oktaAssignments {
		err := ap.DeleteOktaAssignment(ctx, oktaAssignment.GetName())
		require.NoError(t, err)
	}
}

func upsertAccessLists(t *testing.T, ctx context.Context, ap services.AccessLists, accessLists []*accesslist.AccessList) {
	t.Helper()

	for _, accessList := range accessLists {
		_, err := ap.UpsertAccessList(ctx, accessList)
		require.NoError(t, err)
	}
}

func deleteAccessLists(t *testing.T, ctx context.Context, ap services.AccessLists, accessLists []*accesslist.AccessList) {
	t.Helper()

	for _, accessList := range accessLists {
		err := ap.DeleteAccessList(ctx, accessList.GetName())
		if !trace.IsNotFound(err) {
			require.NoError(t, err)
		}
	}
}

func newRole(t *testing.T, name, origin string) types.Role {
	t.Helper()

	role, err := types.NewRole(name, types.RoleSpecV6{})
	require.NoError(t, err)
	if origin != "" {
		role.SetStaticLabels(map[string]string{
			types.OriginLabel: origin,
		})

	}
	return role
}

func upsertRoles(t *testing.T, ctx context.Context, ap services.Access, roles ...types.Role) {
	t.Helper()

	for _, role := range roles {
		_, err := ap.UpsertRole(ctx, role)
		require.NoError(t, err)
	}
}

func deleteRoles(t *testing.T, ctx context.Context, ap services.Access, roles []types.Role) {
	t.Helper()

	for _, role := range roles {
		err := ap.DeleteRole(ctx, role.GetName())
		if !trace.IsNotFound(err) {
			require.NoError(t, err)
		}
	}
}
