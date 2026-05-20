package okta

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

// TestFilteredAppAndGroupSync verifies when bidirectional sync is enabled and the
// sync filters are narrowed to exclude apps/groups that the users in the apps/groups
// are not removed in Okta.
func TestFilteredAppAndGroupSync(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	// Create Okta users.
	user1 := fakeOkta.CreateUser("test-okta-user-1")
	user2 := fakeOkta.CreateUser("test-okta-user-2")
	user3 := fakeOkta.CreateUser("test-okta-user-3")

	// Create Okta groups.
	group1 := fakeOkta.CreateBuiltInGroup("group-1")
	group2 := fakeOkta.CreateBuiltInGroup("group-2")

	// Add users to groups.
	fakeOkta.AddUserToGroup(group1.Id, user1.Id)
	fakeOkta.AddUserToGroup(group1.Id, user2.Id)
	fakeOkta.AddUserToGroup(group2.Id, user3.Id)

	// Assign the users to the SAML app.
	require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, user1.Id))
	require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, user2.Id))
	require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, user3.Id))

	// Setup Teleport.
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)
	oktaAuthClient := sut.GetOktaAuthClient(t, "alice-admin")
	authServer := sut.Teleport.Process.GetAuthServer()

	// Create the integration with App, Group, and Access List sync and bidirectional sync enabled
	integrationSettings := integrationSettings{
		enableUserSync:          true,
		enableAppGroupSync:      true,
		enableAccessListSync:    true,
		enableBidirectionalSync: true,
		accessListSettings: &oktav1.AccessListSettings{
			GroupFilters: []string{"group-*"},
			AppFilters:   []string{"app-*"},
			DefaultOwner: []string{"alice-admin"},
		},
	}

	mustCreateIntegration(t, sut, oktaAuthClient, createIntegrationSettings{
		integrationSettings: integrationSettings,
		apiCredentials:      apiCredentials,
		reuseConnector:      "okta-pre-created-test",
	})

	updateOktaPlugin(t, sut.Teleport.Process.GetAuthServer(), func(p *types.PluginV1) {
		assignmentProcessLoopDuration := time.Millisecond * 300
		p.Spec.GetOkta().SyncSettings.TimeBetweenAssignmentProcessLoops = assignmentProcessLoopDuration.String()
	})

	// 3 test users + alice-admin + approval-bot
	requireOktaUsersSynced(t, 5, authServer)

	t.Run("verify groups synced", func(t *testing.T) {
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			userGroups, _, err := authServer.ListUserGroups(ctx, 0, "")
			require.NoError(t, err)
			require.Len(t, userGroups, 2)
			require.NotNil(t, selectUserGroupByName(userGroups, group1.Id))
			require.NotNil(t, selectUserGroupByName(userGroups, group2.Id))
		}, time.Second*20, time.Millisecond*50)
	})

	t.Run("verify Access Lists synced", func(t *testing.T) {
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			accessLists, err := authServer.GetAccessLists(ctx)
			require.NoError(t, err)
			require.Len(t, accessLists, 2)
		}, time.Second*20, time.Millisecond*50)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			assertAccessListMembers(t, ctx, sut, group1.Id, []string{
				oktaUserLogin(user1),
				oktaUserLogin(user2),
			})
		}, time.Second*20, time.Millisecond*50)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			assertAccessListMembers(t, ctx, sut, group2.Id, []string{
				oktaUserLogin(user3),
			})
		}, time.Second*20, time.Millisecond*50)
	})

	t.Run("make sure narrowed sync filter creates and processes assignments", func(t *testing.T) {
		updateOktaPlugin(t, sut.Teleport.Process.GetAuthServer(), func(p *types.PluginV1) {
			p.Spec.GetOkta().SyncSettings.GroupFilters = []string{"group-2"}
			p.Spec.GetOkta().SyncSettings.AppFilters = []string{"nonexistent-app"}
		})

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			assignments, err := stream.Collect(clientutils.Resources(ctx, authServer.ListOktaAssignments))
			require.NoError(t, err)
			require.Len(t, assignments, 1)
		}, time.Second*20, time.Millisecond*100)
	})

	t.Run("make sure narrowed sync filter Access Lists are updated", func(t *testing.T) {
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			// Only 1 access list present (group-2).
			accessLists, err := authServer.GetAccessLists(ctx)
			require.NoError(t, err)
			require.Len(t, accessLists, 1)
		}, time.Second*20, time.Millisecond*100)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			// group-2 Access List unaffected.
			assertAccessListMembers(t, ctx, sut, group2.Id, []string{
				oktaUserLogin(user3),
			})
		}, time.Second*20, time.Millisecond*100)
	})

	t.Run("make sure narrowed sync filter assignments do not sync back to Okta", func(t *testing.T) {
		require.Equal(t, 2, fakeOkta.GroupAssignments(group1.Id))
		require.Equal(t, 1, fakeOkta.GroupAssignments(group2.Id))
		require.True(t, fakeOkta.IsUserAssignedToApplication(fakeOkta.provisionedSAMLApp.Id, user1.Id))
		require.True(t, fakeOkta.IsUserAssignedToApplication(fakeOkta.provisionedSAMLApp.Id, user2.Id))
		require.True(t, fakeOkta.IsUserAssignedToApplication(fakeOkta.provisionedSAMLApp.Id, user3.Id))
	})
}

func requireOktaUsersSynced(t *testing.T, userCount int, authServer *auth.Server) {
	t.Helper()
	ctx := t.Context()

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		users, err := authServer.GetUsers(ctx, false /* withSecrets */)
		require.NoError(t, err)
		require.Len(t, users, userCount)
	}, time.Second*20, time.Millisecond*50)
}
