package okta

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
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

	// Create the integration with App and Group sync with bidirectional sync disabled

	_, err = oktaAuthClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
		ApiCredentials:          apiCredentials,
		ReuseConnector:          "okta-pre-created-test",
		EnableUserSync:          true,
		EnableAppGroupSync:      true,
		EnableAccessListSync:    false,
		EnableBidirectionalSync: false, // disabled
	})
	require.NoError(t, err)

	// Verify user synced.

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

	t.Run("create access request to group1 and wait for the okta_assignment for it", func(t *testing.T) {
		accessRequest := createAccessRequest(t, sut, group1.Id, types.KindUserGroup, oktaUserEmail)
		approveAccessRequest(t, sut, accessRequest.GetName(), "alice-admin")

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			_, err := authServer.GetOktaAssignment(ctx, accessRequest.GetName())
			require.NoError(t, err)
		}, time.Second*10, time.Millisecond*50)
	})

	t.Run("make sure there are not assignments to any group", func(t *testing.T) {
		// group 1
		userIDs, err := oktaClient.GetGroupAssignments(ctx, oktaapi.OktaGroupID(group1.Id))
		require.NoError(t, err)
		require.Empty(t, userIDs)
		// group 2
		userIDs, err = oktaClient.GetGroupAssignments(ctx, oktaapi.OktaGroupID(group2.Id))
		require.NoError(t, err)
		require.Empty(t, userIDs)
	})

	// Enable bidirectional sync

	_, err = oktaAuthClient.UpdateIntegration(ctx, &oktav1.UpdateIntegrationRequest{
		EnableUserSync:          true,
		EnableAppGroupSync:      true,
		EnableAccessListSync:    false,
		EnableBidirectionalSync: true, // enabled
	})
	require.NoError(t, err)

	t.Run("verify assignment to the group1 is processed", func(t *testing.T) {
		// group 1
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			userIDs, err := oktaClient.GetGroupAssignments(ctx, oktaapi.OktaGroupID(group1.Id))
			require.NoError(t, err)
			require.Len(t, userIDs, 1)
			require.Contains(t, userIDs, oktaapi.OktaUserID(oktaUser.Id))
		}, time.Second*2, time.Millisecond*50)
		// group 2
		userIDs, err := oktaClient.GetGroupAssignments(ctx, oktaapi.OktaGroupID(group2.Id))
		require.NoError(t, err)
		require.Empty(t, userIDs)
	})
}
