package okta

import (
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
)

// Verifies app and group only sync pushes RBAC changes to Okta and respects the bidirectional sync
// flag.
func Test_AppAndGroup_only_sync(t *testing.T) {
	ctx := t.Context()

	// Setup Okta mock.
	oktaAPIClient := newMockOktaAPIClient("https://trial-1234567.okta.com")
	oktaClient := oktaapi.NewForAPIClient(oktaAPIClient)

	// Create Okta SAML app.
	connectorSamlApp := createOktaSAMLAPP(t, ctx, oktaAPIClient, "trial-1234567_teleportsamlconnectorapp_1")

	// Create Okta user.
	oktaUser, oktaUserEmail := createOktaUser(t, ctx, oktaAPIClient, "test-okta-user-1")

	// Assign the user to the SAML app.
	err := oktaClient.AssignUserToApplication(ctx, oktaapi.OktaUserID(oktaUser.Id), oktaapi.OktaAppID(connectorSamlApp.Id))
	require.NoError(t, err)

	// Create Okta groups.
	group1 := createOktaGroup(t, ctx, oktaAPIClient, "group1")
	group2 := createOktaGroup(t, ctx, oktaAPIClient, "group2")

	// Setup Teleport.
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)
	oktaAuthClient := sut.GetOktaAuthClient(t, "alice-admin")
	authServer := sut.Teleport.Process.GetAuthServer()

	// Create okta-reviewer roles and assign it to alice-admin user.
	oktaReviewerRole := createRole(t, sut, "okta-reviewer",
		roleAllowDesc{
			reviewRequests: &types.AccessReviewConditions{
				Roles: []string{teleport.SystemOktaAccessRoleName},
			},
		},
		roleDenyDesc{},
	)
	assignRoles(t, sut, "alice-admin", oktaReviewerRole.GetName())

	// Create the integration with App and Group only sync and bidirectional sync enabled
	integrationSettings := integrationSettings{
		enableUserSync:          true,
		enableAppGroupSync:      true,
		enableAccessListSync:    false,
		enableBidirectionalSync: true,
	}
	mustCreateIntegration(t, oktaAuthClient, createIntegrationSettings{
		integrationSettings: integrationSettings,
		apiCredentials:      apiCredentials,
		reuseConnector:      "okta-pre-created-test",
	})

	t.Run("verify users synced", func(t *testing.T) {
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			_, err := authServer.GetUser(ctx, oktaUserEmail, false /* withSecrets */)
			require.NoError(t, err)
		}, time.Second*2, time.Millisecond*50)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			uls, err := authServer.GetUserLoginState(ctx, oktaUserEmail)
			assert.NoError(t, err)
			assert.Contains(t, uls.GetRoles(), teleport.SystemOktaRequesterRoleName)
		}, time.Second*2, time.Millisecond*50)
	})

	t.Run("verify groups synced", func(t *testing.T) {
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			userGroups, _, err := authServer.ListUserGroups(ctx, 0, "")
			require.NoError(t, err)
			require.Len(t, userGroups, 2)
			require.NotNil(t, selectUserGroupByName(userGroups, group1.Id))
			require.NotNil(t, selectUserGroupByName(userGroups, group2.Id))
		}, time.Second*2, time.Millisecond*50)
	})

	t.Run("make sure there are not assignments to any group", func(t *testing.T) {
		requireNoGroupAssignments(t, oktaAPIClient, group1.Id)
		requireNoGroupAssignments(t, oktaAPIClient, group2.Id)
	})

	var accessRequestName string

	t.Run("create access request to group1 and wait for the okta_assignment for it", func(t *testing.T) {
		accessRequest := createAccessRequest(t, sut, group1.Id, types.KindUserGroup, oktaUserEmail)
		approveAccessRequest(t, sut, accessRequest.GetName(), "alice-admin")

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			_, err := authServer.GetOktaAssignment(ctx, accessRequest.GetName())
			require.NoError(t, err)
		}, time.Second*10, time.Millisecond*50)

		accessRequestName = accessRequest.GetName()
	})

	t.Run("verify assignment to the group1 is processed", func(t *testing.T) {
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			requireGroupAssignments(t, oktaAPIClient, group1.Id, oktaUser.Id)
		}, time.Second*4, time.Millisecond*50)
		requireNoGroupAssignments(t, oktaAPIClient, group2.Id)
	})

	// Disable bidirectional sync
	integrationSettings.enableBidirectionalSync = false
	mustUpdateIntegration(t, oktaAuthClient, integrationSettings)

	t.Run("delete access request to group1 and wait for the okta_assignment to be marked for cleanup", func(t *testing.T) {
		err := authServer.DeleteAccessRequest(ctx, accessRequestName)
		require.NoError(t, err)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			assignment, err := authServer.GetOktaAssignment(ctx, accessRequestName)
			require.NoError(t, err)
			require.True(t, assignment.GetCleanupTime().Before(time.Now()), "require cleanup time to be set before now")
			require.False(t, assignment.IsFinalized())
		}, time.Second*10, time.Millisecond*50)
	})

	t.Run("verify assignment to the group1 is still there on the Okta side", func(t *testing.T) {
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			requireGroupAssignments(t, oktaAPIClient, group1.Id, oktaUser.Id)
		}, time.Second*2, time.Millisecond*50)
		requireNoGroupAssignments(t, oktaAPIClient, group2.Id)
	})

	// Enable bidirectional sync
	integrationSettings.enableBidirectionalSync = true
	mustUpdateIntegration(t, oktaAuthClient, integrationSettings)

	t.Run("okta_assignment is cleaned up", func(t *testing.T) {
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			_, err := authServer.GetOktaAssignment(ctx, accessRequestName)
			require.Error(t, err)
			require.True(t, trace.IsNotFound(err))
		}, time.Second*10, time.Millisecond*50)
	})

	t.Run("verify assignment to the group1 is cleaned up on the Okta side", func(t *testing.T) {
		requireNoGroupAssignments(t, oktaAPIClient, group1.Id)
		requireNoGroupAssignments(t, oktaAPIClient, group2.Id)
	})
}
