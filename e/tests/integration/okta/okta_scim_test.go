package okta

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	libokta "github.com/gravitational/teleport/e/lib/okta"
	oktacommon "github.com/gravitational/teleport/e/lib/okta/common"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
)

func TestSCIMAuth(t *testing.T) {
	oktaAppGroupsUsersCount := withAppsGroupsUsersCount(3, 3, 3)
	mockClient := newMockOktaAPIClient("https://trial-1234567.okta.com")
	_ = createOktaSetup(t, t.Context(), mockClient, oktaAppGroupsUsersCount)
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(&muxToTransportWrapper{handler: setupOktaAPIServerForSCIMFlow(t, mockClient)}),
	)
	validToken := createAndWaitForOktaIntegration(t, sut, mockClient)

	scimUser := &scimsdk.User{ExternalID: "alice", UserName: "alice@example.com", Active: true}
	t.Run("request should fail with invalid token", func(t *testing.T) {
		scimClient := createSCIMClient(t, sut, "invalid-token")
		_, err := scimClient.CreateUser(t.Context(), scimUser)
		require.Error(t, err)
	})
	t.Run("request should succeed with valid token", func(t *testing.T) {
		scimClient := createSCIMClient(t, sut, validToken)
		u, err := scimClient.CreateUser(t.Context(), scimUser)
		require.NoError(t, err)
		// Cleanup user state for further flow.
		// Using Okta peculiar behavior to deactivate the user instead of deleting.
		user, err := scimClient.GetUser(t.Context(), u.ID)
		require.NoError(t, err)

		user.Attributes = scimsdk.AttributeSet{"active": false}
		_, err = scimClient.UpdateUser(t.Context(), user)
		require.NoError(t, err)
	})
}

func TestSCIMCRUD(t *testing.T) {
	ctx := context.Background()
	oktaAppGroupsUsersCount := withAppsGroupsUsersCount(3, 3, 3)
	mockClient := newMockOktaAPIClient("https://trial-1234567.okta.com")
	_ = createOktaSetup(t, ctx, mockClient, oktaAppGroupsUsersCount)
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(&muxToTransportWrapper{handler: setupOktaAPIServerForSCIMFlow(t, mockClient)}),
	)
	validToken := createAndWaitForOktaIntegration(t, sut, mockClient)

	testSCIMCRUD(t, mockClient, createSCIMClient(t, sut, validToken))
}

func testSCIMCRUD(t *testing.T, infraClient *mockOktaAPIClient, client scimsdk.Client) {
	ctx := context.Background()
	scimUserName := "test-user+001@example.com"
	scimUserExternalID := "test-user-001"
	t.Run("Create SCIM User", func(t *testing.T) {
		scimUser := &scimsdk.User{ExternalID: scimUserExternalID, UserName: scimUserName, Active: true}
		createdUser, err := client.CreateUser(ctx, scimUser)
		require.NoError(t, err)
		require.NotNil(t, createdUser)
		assertSCIMUserSchema(t, createdUser)
	})

	t.Run("Get SCIM User", func(t *testing.T) {
		user, err := client.GetUser(ctx, scimUserName)
		require.NoError(t, err)
		require.Equal(t, scimUserName, user.UserName)
		assertSCIMUserSchema(t, user)
	})

	t.Run("Update SCIM User", func(t *testing.T) {
		user, err := client.GetUser(ctx, scimUserName)
		require.NoError(t, err)
		updatedUser, err := client.UpdateUser(ctx, user)
		require.NoError(t, err)
		require.NotNil(t, updatedUser)

		t.Run("should handle nil values", func(t *testing.T) {
			user, err = client.GetUser(ctx, scimUserName)
			require.NoError(t, err)
			user.Meta = nil
			_, err = client.UpdateUser(ctx, user)
			require.Error(t, err)

			updatedUser.Name = nil
			updatedUser, err = client.UpdateUser(ctx, updatedUser)
			require.NoError(t, err)
			assertSCIMUserSchema(t, updatedUser)
		})
	})

	t.Run("Delete SCIM User", func(t *testing.T) {
		err := client.DeleteUser(ctx, scimUserName)
		// Okta SCIM API does not support user deletion.
		require.Error(t, err)
		// Instead of deleting the user, we deactivate the user.
		user, err := client.GetUser(ctx, scimUserName)
		require.NoError(t, err)
		user.Attributes = scimsdk.AttributeSet{"active": false}
		_, err = client.UpdateUser(ctx, user)
		require.NoError(t, err)

		// After deactivation the user should delete from downstream system
		// and should not be found.
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			user, err = client.GetUser(ctx, scimUserName)
			require.True(t, trace.IsBadParameter(err))
		}, time.Second*3, time.Millisecond*50)
	})

	var group *scimsdk.Group
	var groupName = "Test SCIM Group1"
	t.Run("Create SCIM Group", func(t *testing.T) {
		_, _, err := infraClient.CreateGroup(ctx, okta.Group{
			Type:    "OKTA_GROUP",
			Profile: &okta.GroupProfile{Name: groupName},
		})
		assert.NoError(t, err)
		scimGroups := []*scimsdk.Group{{DisplayName: groupName}}
		groups := provisionSCIMGroups(t, client, scimGroups)
		require.Len(t, groups, 1)
		group = groups[0]
		assertSCIMGroupSchema(t, group)
	})

	t.Run("Get SCIM Group", func(t *testing.T) {
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			got, err := client.GetGroupByDisplayName(ctx, groupName)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, groupName, got.DisplayName)
		}, time.Second*3, time.Millisecond*50)
	})

	t.Run("Update SCIM Group", func(t *testing.T) {
		g, err := client.GetGroupByDisplayName(ctx, groupName)
		require.NoError(t, err)
		assertSCIMGroupSchema(t, g)

		g.DisplayName = "Updated SCIM Group"
		updatedGroup, err := client.UpdateGroup(ctx, g)
		require.NoError(t, err)
		assertSCIMGroupSchema(t, updatedGroup)

		got, err := client.GetGroup(ctx, g.ID)
		require.NoError(t, err)
		require.Equal(t, "Updated SCIM Group", got.DisplayName)
		assertSCIMGroupSchema(t, got)

		t.Run("should handle nil values", func(t *testing.T) {
			g, err := client.GetGroup(ctx, g.ID)
			require.NoError(t, err)
			g.Meta = nil
			updatedGroup, err = client.UpdateGroup(ctx, g)
			require.NoError(t, err)
			assertSCIMGroupSchema(t, updatedGroup)
		})
	})

	t.Run("Delete SCIM Group", func(t *testing.T) {
		g, err := client.GetGroup(ctx, group.ID)
		require.NoError(t, err)
		err = client.DeleteGroup(ctx, g.ID)
		require.NoError(t, err)

		_, err = client.GetGroup(ctx, g.ID)
		require.Error(t, err)
	})
}

func TestSCIMOktaUserProvisioning(t *testing.T) {
	ctx := context.Background()

	oktaAppGroupsUsersCount := withAppsGroupsUsersCount(3, 3, 3)
	mockClient := newMockOktaAPIClient("https://trial-1234567.okta.com")
	_ = createOktaSetup(t, ctx, mockClient, oktaAppGroupsUsersCount)
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(&muxToTransportWrapper{handler: setupOktaAPIServerForSCIMFlow(t, mockClient)}),
	)

	scimToken := createAndWaitForOktaIntegration(t, sut, mockClient)
	scimClient := createSCIMClient(t, sut, scimToken)

	scimUsers := provisionSCIMUsers(t, scimClient, "001", "002", "003")
	testUserDeactivationActivation(t, ctx, sut, scimClient, scimUsers[0].ID)
}

func TestSCIMUserOktaProvisioningWithoutAccessListSyncDisabled(t *testing.T) {
	ctx := context.Background()

	oktaAppGroupsUsersCount := withAppsGroupsUsersCount(3, 3, 3)
	mockClient := newMockOktaAPIClient("https://trial-1234567.okta.com")
	_ = createOktaSetup(t, ctx, mockClient, oktaAppGroupsUsersCount)
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(&muxToTransportWrapper{handler: setupOktaAPIServerForSCIMFlow(t, mockClient)}),
	)

	scimToken := createAndWaitForOktaIntegration(t, sut, mockClient, withAccessListDisabled())
	scimClient := createSCIMClient(t, sut, scimToken)

	scimUsers := provisionSCIMUsers(t, scimClient, "001", "002", "003")
	testUserDeactivationActivation(t, ctx, sut, scimClient, scimUsers[0].ID)
}

func TestSCIMOktaGroupProvisioning(t *testing.T) {
	ctx := context.Background()
	mockClient := newMockOktaAPIClient("https://trial-1234567.okta.com")

	oktaAppGroupsUsersCount := withAppsGroupsUsersCount(3, 3, 3)
	_ = createOktaSetup(t, ctx, mockClient, oktaAppGroupsUsersCount)
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(&muxToTransportWrapper{handler: setupOktaAPIServerForSCIMFlow(t, mockClient)}),
	)

	scimToken := createAndWaitForOktaIntegration(t, sut, mockClient)
	scimClient := createSCIMClient(t, sut, scimToken)

	scimUsers := provisionSCIMUsers(t, scimClient, "001", "002", "003")

	scimGroup, _, err := mockClient.CreateGroup(context.Background(), okta.Group{
		Type:    "OKTA_GROUP",
		Profile: &okta.GroupProfile{Name: "Test SCIM Group1"},
	})
	require.NoError(t, err)

	t.Run("SCIM group provisioning should create ACL", func(t *testing.T) {
		var scimGroups = []*scimsdk.Group{
			{
				DisplayName: scimGroup.Profile.Name,
				Members:     []*scimsdk.GroupMember{{ExternalID: scimUsers[0].ID}},
			},
		}
		provisionSCIMGroups(t, scimClient, scimGroups)

		acl, err := sut.Teleport.Process.GetAuthServer().GetAccessList(ctx, scimGroup.Id)
		require.NoError(t, err)

		require.Equal(t, acl.Spec.Title, scimGroup.Profile.Name)
		require.Equal(t, []string{oktacommon.CreateOktaAccessRoleFriendlyName(scimGroup.Profile.Name, scimGroup.Id)}, acl.Spec.Grants.Roles)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			members, _, err := sut.Teleport.Process.GetAuthServer().ListAccessListMembers(ctx, scimGroup.Id, 0, "")
			assert.NoError(t, err)
			if !assert.Len(t, members, 1) {
				return
			}
			assert.Equal(t, scimUsers[0].ID, members[0].GetName())
		}, 10*time.Second, time.Millisecond*50)
	})

	t.Run("SCIM group update should update ACL membership", func(t *testing.T) {
		group, err := scimClient.GetGroup(ctx, scimGroup.Id)
		require.NoError(t, err)
		require.Equal(t, group.DisplayName, scimGroup.Profile.Name)
		group.Members = append(group.Members, &scimsdk.GroupMember{
			ExternalID: scimUsers[1].ID,
		})
		_, err = scimClient.UpdateGroup(ctx, group)
		require.NoError(t, err)
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			members, _, err := sut.Teleport.Process.GetAuthServer().ListAccessListMembers(ctx, scimGroup.Id, 0, "")
			assert.NoError(t, err)
			assert.Len(t, members, 2)
		}, time.Second, time.Millisecond*50)
	})

	t.Run("SCIM group deprovisioning should delete ACL", func(t *testing.T) {
		err = scimClient.DeleteGroup(ctx, scimGroup.Id)
		require.NoError(t, err)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			_, err = sut.Teleport.Process.GetAuthServer().GetAccessList(ctx, scimGroup.Id)
			assert.True(t, trace.IsNotFound(err))
		}, time.Second, time.Millisecond*50)
	})
}

func testUserDeactivationActivation(t *testing.T, ctx context.Context, sut *common.SUT, client scimsdk.Client, username string) {
	auth := sut.Teleport.Process.GetAuthServer()
	user, err := client.GetUserByUserName(ctx, username)
	require.NoError(t, err)

	user.Attributes = scimsdk.AttributeSet{"active": false}
	_, err = client.UpdateUser(ctx, user)
	require.NoError(t, err)

	// When user is deactivated the user should be removed from teleport backend
	// and a user lock should be created to kill all teleport active sessions.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		userLocks, err := auth.GetLocks(ctx, false, types.LockTarget{User: user.ID})
		assert.NoError(t, err)
		if !assert.Len(t, userLocks, 1) {
			return
		}
		assert.Equal(t, types.OriginOkta, userLocks[0].Origin())
		assert.Equal(t, libokta.LockReasonDeactivated, userLocks[0].GetAllLabels()[teleport.OktaLockReasonLabel])
	}, time.Second, time.Millisecond*40)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		_, err = auth.GetUser(ctx, user.ID, false)
		require.True(t, trace.IsNotFound(err))
	}, time.Second, time.Millisecond*40)

	// Recover user by settings the user to active state. And recreate the user.
	// User recreation is peculiar Okta behavior, when user is recovered from deactivated state  Okta will
	//  create a new user instead of updating the existing user.
	user.Attributes = scimsdk.AttributeSet{"active": true}
	_, err = client.CreateUser(ctx, user)
	require.NoError(t, err)

	_, err = auth.GetUser(ctx, user.ID, false)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		// If a user was re-activated, the lock created by deactivation flow should be removed.
		userLocks, err := sut.Teleport.Process.GetAuthServer().GetLocks(ctx, false, types.LockTarget{
			User: user.ID,
		})
		assert.NoError(t, err)
		assert.Empty(t, userLocks)
	}, time.Second, time.Millisecond*40)
}
