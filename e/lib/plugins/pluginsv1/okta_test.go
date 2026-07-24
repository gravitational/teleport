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
	"github.com/gravitational/teleport/api/defaults"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
)

const (
	testHostname = "test-host"
	testHostID   = "test-host-id"
)

func TestService_CleanupOkta(t *testing.T) {
	t.Parallel()

	suite := createSuite(t)
	suite.setRules([]types.Rule{
		{Resources: []string{types.KindPlugin}, Verbs: services.RW()},
	})

	ctx := context.Background()

	expectedNeedsCleanup := func(active bool, resourcesToCleanup ...*types.ResourceID) {
		resp, err := suite.svc.NeedsCleanup(ctx, pluginspb.NeedsCleanupRequest_builder{
			Type: types.PluginTypeOkta,
		}.Build())
		require.NoError(t, err)
		require.Equal(t, len(resourcesToCleanup) > 0, resp.GetNeedsCleanup())
		require.Equal(t, resourcesToCleanup, resp.GetResourcesToCleanup())
		require.Equal(t, active, resp.GetPluginActive())
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
	appServersToCleanup := []types.AppServer{
		newAppServer(t, "app-server-1-cleanup", types.OriginOkta),
		newAppServer(t, "app-server-2-cleanup", types.OriginOkta),
		newAppServer(t, "app-server-3-cleanup", types.OriginOkta),
	}
	appServers := []types.AppServer{
		newAppServer(t, "app-server-1", ""),
		newAppServer(t, "app-server-2", ""),
		newAppServer(t, "app-server-3", ""),
	}
	userGroupsToCleanup := []types.UserGroup{
		newUserGroup(t, "user-group-1-cleanup", types.OriginOkta),
		newUserGroup(t, "user-group-2-cleanup", types.OriginOkta),
		newUserGroup(t, "user-group-3-cleanup", types.OriginOkta),
	}
	userGroups := []types.UserGroup{
		newUserGroup(t, "user-group-1", ""),
		newUserGroup(t, "user-group-2", ""),
		newUserGroup(t, "user-group-3", ""),
	}
	rolesToCleanup := []types.Role{
		newRole(t, "r1-cleanup", types.OriginOkta),
		newRole(t, "r2-cleanup", types.OriginOkta),
		newRole(t, "r3-cleanup", types.OriginOkta),
	}
	oktaAccessRole := services.NewSystemOktaAccessRole(modules.BuildEnterprise)
	oktaRequesterRole := services.NewSystemOktaRequesterRole(modules.BuildEnterprise)
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

	// Make sure app servers can cause a needs cleanup.
	upsertAppServers(t, suite.svc.authServer, appServers)
	expectedNeedsCleanup(false)
	upsertAppServers(t, suite.svc.authServer, appServersToCleanup)
	expectedNeedsCleanup(false,
		&types.ResourceID{Kind: types.KindAppServer, Name: testHostID + "/app-server-1-cleanup"},
		&types.ResourceID{Kind: types.KindAppServer, Name: testHostID + "/app-server-2-cleanup"},
		&types.ResourceID{Kind: types.KindAppServer, Name: testHostID + "/app-server-3-cleanup"},
	)
	deleteAppServers(t, suite.svc.authServer, appServersToCleanup)

	// Make sure user groups can cause a needs cleanup.
	upsertUserGroups(t, suite.svc.authServer, userGroups)
	expectedNeedsCleanup(false)
	upsertUserGroups(t, suite.svc.authServer, userGroupsToCleanup)
	expectedNeedsCleanup(false,
		&types.ResourceID{Kind: types.KindUserGroup, Name: "user-group-1-cleanup"},
		&types.ResourceID{Kind: types.KindUserGroup, Name: "user-group-2-cleanup"},
		&types.ResourceID{Kind: types.KindUserGroup, Name: "user-group-3-cleanup"},
	)
	deleteUserGroups(t, suite.svc.authServer, userGroupsToCleanup)

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
	upsertRoles(t, ctx, suite.svc.authServer, services.NewSystemOktaRequesterRole(modules.BuildEnterprise))

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
	_, err := suite.svc.CreatePlugin(ctx, pluginspb.CreatePluginRequest_builder{
		Plugin:            oktaPlugin,
		StaticCredentials: staticCredentials,
	}.Build())
	require.NoError(t, err)

	expectedNeedsCleanup(true)

	// Put all of the resources that need a cleanup back in.
	upsertOktaAssignments(t, ctx, suite.svc.authServer, oktaAssignments)
	upsertAccessLists(t, ctx, suite.svc.authServer, accessListsToCleanup)
	upsertAppServers(t, suite.svc.authServer, appServersToCleanup)
	upsertUserGroups(t, suite.svc.authServer, userGroupsToCleanup)
	upsertRoles(t, ctx, suite.svc.authServer, rolesToCleanup...)
	oktaRequesterRole.SetSearchAsRoles(types.Allow, []string{"r1-cleanup", "r2-cleanup", "r3-cleanup"})
	upsertRoles(t, ctx, suite.svc.authServer, oktaRequesterRole)

	resourcesExpectedToCleanup := []*types.ResourceID{
		{Kind: types.KindOktaAssignment, Name: "assignment1"},
		{Kind: types.KindOktaAssignment, Name: "assignment2"},
		{Kind: types.KindOktaAssignment, Name: "assignment3"},
		{Kind: types.KindAccessList, Name: "al1-cleanup"},
		{Kind: types.KindAccessList, Name: "al2-cleanup"},
		{Kind: types.KindAccessList, Name: "al3-cleanup"},
		{Kind: types.KindAppServer, Name: testHostID + "/app-server-1-cleanup"},
		{Kind: types.KindAppServer, Name: testHostID + "/app-server-2-cleanup"},
		{Kind: types.KindAppServer, Name: testHostID + "/app-server-3-cleanup"},
		{Kind: types.KindUserGroup, Name: "user-group-1-cleanup"},
		{Kind: types.KindUserGroup, Name: "user-group-2-cleanup"},
		{Kind: types.KindUserGroup, Name: "user-group-3-cleanup"},
		{Kind: types.KindRole, Name: "r1-cleanup"},
		{Kind: types.KindRole, Name: "r2-cleanup"},
		{Kind: types.KindRole, Name: "r3-cleanup"},
		{Kind: types.KindRole, Name: teleport.SystemOktaRequesterRoleName},
	}

	// The plugin is active.
	expectedNeedsCleanup(true, resourcesExpectedToCleanup...)

	// Cleanup fails due to an active plugin.
	_, err = suite.svc.Cleanup(ctx, pluginspb.CleanupRequest_builder{
		Type: types.PluginTypeOkta,
	}.Build())
	require.ErrorContains(t, err, "can't cleanup")

	// Delete the plugin, which means the plugin should no longer be active.
	_, err = suite.svc.DeletePlugin(ctx, pluginspb.DeletePluginRequest_builder{
		Name: oktaPlugin.GetName(),
	}.Build())
	require.NoError(t, err)

	// The plugin is inactive.
	expectedNeedsCleanup(false, resourcesExpectedToCleanup...)

	// Cleanup should succeed.
	_, err = suite.svc.Cleanup(ctx, pluginspb.CleanupRequest_builder{
		Type: types.PluginTypeOkta,
	}.Build())
	require.NoError(t, err)

	backendOktaAssignments, _, err := suite.svc.authServer.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)
	backendAccessLists, _, err := suite.svc.authServer.ListAccessLists(ctx, 0, "")
	require.NoError(t, err)
	backendAppServers, err := suite.svc.authServer.GetApplicationServers(ctx, defaults.Namespace)
	require.NoError(t, err)
	backendUserGroups, _, err := suite.svc.authServer.ListUserGroups(ctx, 0, "")
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
	roles[1].SetSearchAsRoles(types.Allow, services.NewSystemOktaRequesterRole(modules.BuildEnterprise).GetSearchAsRoles(types.Allow))

	require.Empty(t, backendOktaAssignments)
	require.Empty(t, cmp.Diff(accessLists, backendAccessLists, cmpopts.IgnoreFields(header.Metadata{}, "Revision")))
	require.Empty(t, cmp.Diff(appServers, backendAppServers, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	require.Empty(t, cmp.Diff(userGroups, backendUserGroups, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	require.Empty(t, cmp.Diff(roles, backendRoles, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
}

func Test_oktaHandler_validatePlugin(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name         string
		oktaSettings *types.PluginOktaSettings
		errMatcher   func(error) bool
		errContains  string
	}

	for _, tt := range []testCase{
		{
			name: "malformed TimeBetweenImports",
			oktaSettings: &types.PluginOktaSettings{
				SyncSettings: &types.PluginOktaSyncSettings{
					TimeBetweenImports: "not_a_duration",
				},
			},
			errMatcher:  trace.IsBadParameter,
			errContains: "time_between_imports is not valid",
		},
		{
			name: "malformed TimeBetweenAssignmentProcessLoops",
			oktaSettings: &types.PluginOktaSettings{
				SyncSettings: &types.PluginOktaSyncSettings{
					TimeBetweenAssignmentProcessLoops: "not_a_duration",
				},
			},
			errMatcher:  trace.IsBadParameter,
			errContains: "time_between_assignment_process_loops is not valid",
		},
		{
			name: "TimeBetweenAssignmentProcessLoops longer than TimeBetweenImports",
			oktaSettings: &types.PluginOktaSettings{
				SyncSettings: &types.PluginOktaSyncSettings{
					TimeBetweenImports:                "1m",
					TimeBetweenAssignmentProcessLoops: "1m6s",
				},
			},
			errMatcher:  trace.IsBadParameter,
			errContains: "time_between_assignment_process_loops cannot be longer than time_between_imports",
		},
		{
			name: "TimeBetweenAssignmentProcessLoops longer than implicit TimeBetweenImports",
			oktaSettings: &types.PluginOktaSettings{
				SyncSettings: &types.PluginOktaSyncSettings{
					// TimeBetweenImports is 30m by default
					TimeBetweenAssignmentProcessLoops: "30m1s",
				},
			},
			errMatcher:  trace.IsBadParameter,
			errContains: "time_between_assignment_process_loops cannot be longer than time_between_imports",
		},
		{
			name: "malformed TargetProcessingBackoffStep",
			oktaSettings: &types.PluginOktaSettings{
				SyncSettings: &types.PluginOktaSyncSettings{
					TargetProcessingBackoffStep: "not_a_duration",
				},
			},
			errMatcher:  trace.IsBadParameter,
			errContains: "target_processing_backoff_step is not valid",
		},
		{
			name: "negative TargetProcessingBackoffStep",
			oktaSettings: &types.PluginOktaSettings{
				SyncSettings: &types.PluginOktaSyncSettings{
					TargetProcessingBackoffStep: "-5m",
				},
			},
			errMatcher:  trace.IsBadParameter,
			errContains: "target_processing_backoff_step \"-5m\" cannot be a negative value",
		},
		{
			name: "malformed TargetProcessingBackoffMax",
			oktaSettings: &types.PluginOktaSettings{
				SyncSettings: &types.PluginOktaSyncSettings{
					TargetProcessingBackoffMax: "not_a_duration",
				},
			},
			errMatcher:  trace.IsBadParameter,
			errContains: "target_processing_backoff_max is not valid",
		},
		{
			name: "negative TargetProcessingBackoffMax",
			oktaSettings: &types.PluginOktaSettings{
				SyncSettings: &types.PluginOktaSyncSettings{
					TargetProcessingBackoffMax: "-5m",
				},
			},
			errMatcher:  trace.IsBadParameter,
			errContains: "target_processing_backoff_max \"-5m\" cannot be a negative value",
		},
		{
			name: "TargetProcessingBackoffStep longer than TargetProcessingBackoffMax",
			oktaSettings: &types.PluginOktaSettings{
				SyncSettings: &types.PluginOktaSyncSettings{
					TargetProcessingBackoffStep: "15m",
					TargetProcessingBackoffMax:  "5m",
				},
			},
			errMatcher:  trace.IsBadParameter,
			errContains: "target_processing_backoff_max \"5m0s\" must be longer than target_processing_backoff_step \"15m0s\"",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			plugin := newTestOktaPlugin(tt.oktaSettings)
			err := oktaPluginHandler{}.validatePlugin(t.Context(), pluginValidationInput{plugin: plugin}, nil)
			if tt.errMatcher == nil && tt.errContains == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.errContains)
			require.True(t, tt.errMatcher(err))
		})
	}
}

func newTestOktaPlugin(oktaSettings *types.PluginOktaSettings) *types.PluginV1 {
	return types.NewPluginV1(
		types.Metadata{
			Name: types.PluginTypeOkta,
		},
		types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Okta{
				Okta: oktaSettings,
			},
		},
		nil,
	)
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

func newAppServer(t *testing.T, name, origin string) types.AppServer {
	t.Helper()

	var labels map[string]string
	if origin != "" {
		labels = map[string]string{}
		labels[types.OriginLabel] = origin
	}

	app, err := types.NewAppV3(
		types.Metadata{
			Name:   name,
			Labels: labels,
		},
		types.AppSpecV3{
			URI: "https://www.link1.com",
		},
	)
	require.NoError(t, err)

	appServer, err := types.NewAppServerV3(
		app.GetMetadata(),
		types.AppServerSpecV3{
			App:      app,
			Hostname: testHostname,
			HostID:   testHostID,
		},
	)
	require.NoError(t, err)

	return appServer
}

func newUserGroup(t *testing.T, name, origin string) types.UserGroup {
	t.Helper()

	var labels map[string]string
	if origin != "" {
		labels = map[string]string{}
		labels[types.OriginLabel] = origin
	}

	group, err := types.NewUserGroup(
		types.Metadata{
			Name:   name,
			Labels: labels,
		},
		types.UserGroupSpecV1{},
	)
	require.NoError(t, err)

	return group
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

func upsertAppServers(t *testing.T, ap services.Presence, appServers []types.AppServer) {
	t.Helper()
	ctx := t.Context()

	for _, as := range appServers {
		_, err := ap.UpsertApplicationServer(ctx, as)
		require.NoError(t, err)
	}
}

func deleteAppServers(t *testing.T, ap services.Presence, appServers []types.AppServer) {
	t.Helper()
	ctx := t.Context()

	for _, as := range appServers {
		err := ap.DeleteAppServer(ctx, presencev1.DeleteAppServerRequest_builder{HostId: as.GetHostID(), Name: as.GetName()}.Build())
		require.NoError(t, err)
	}
}

func upsertUserGroups(t *testing.T, ap services.UserGroups, userGroups []types.UserGroup) {
	t.Helper()
	ctx := t.Context()

	for _, g := range userGroups {
		upsertErr := ap.CreateUserGroup(ctx, g)
		if trace.IsAlreadyExists(upsertErr) {
			current, err := ap.GetUserGroup(ctx, g.GetName())
			require.NoError(t, err)

			g.SetRevision(current.GetRevision())
			upsertErr = ap.UpdateUserGroup(ctx, g)
		}
		require.NoError(t, upsertErr)
	}
}

func deleteUserGroups(t *testing.T, ap services.UserGroups, userGroups []types.UserGroup) {
	t.Helper()
	ctx := t.Context()

	for _, g := range userGroups {
		err := ap.DeleteUserGroup(ctx, g.GetName())
		require.NoError(t, err)
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
