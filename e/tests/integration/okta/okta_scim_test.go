package okta

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gravitational/trace"
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
	t.Parallel()

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
		withUserCount(3),
		withAppCount(3),
		withGroupCount(3),
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)
	validToken := createAndWaitForOktaIntegration(t, sut, fakeOkta)

	scimUser := &scimsdk.User{ExternalID: "alice", UserName: "alice@example.com", Active: true}
	t.Run("request should fail with invalid token", func(t *testing.T) {
		// invalid token
		scimClient := createSCIMClient(t, sut, "invalid-token")
		_, err := scimClient.CreateUser(t.Context(), scimUser)
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err), "error type = %T", err)

		// malformed token
		req, err := http.NewRequest("GET", scimBaseURL(sut)+"/Users", nil)
		require.NoError(t, err)
		req.Header.Add("Authorization", "non-token")
		resp, err := newInsecureHTTPClient().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, 401, resp.StatusCode)
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
	t.Parallel()

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
		withUserCount(3),
		withAppCount(3),
		withGroupCount(3),
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)
	validToken := createAndWaitForOktaIntegration(t, sut, fakeOkta)

	testSCIMCRUD(t, fakeOkta, createSCIMClient(t, sut, validToken))
}

func testSCIMCRUD(t *testing.T, fakeOkta *fakeOktaServer, client scimsdk.Client) {
	ctx := t.Context()
	scimUserName := "test-user+001@example.com"
	scimUserExternalID := "test-user-001"
	scimUserExtraAttrs := scimsdk.AttributeSet{
		"arbitrary_attr_name":   []any{"value"},
		"arbitrary_attr_number": float64(8),
	}
	scimUser := &scimsdk.User{ExternalID: scimUserExternalID, UserName: scimUserName, Active: true, Attributes: scimUserExtraAttrs}

	t.Run("Create SCIM User", func(t *testing.T) {
		createdUser, err := client.CreateUser(ctx, scimUser)
		require.NoError(t, err)
		require.NotNil(t, createdUser)
		assertSCIMUserSchema(t, createdUser)
		require.Equal(t, scimUserExtraAttrs, createdUser.Attributes)
	})

	t.Run("Create SCIM User that already exists is an error", func(t *testing.T) {
		_, err := client.CreateUser(t.Context(), scimUser)
		require.Error(t, err)
		require.True(t, trace.IsAlreadyExists(err), "error type = %T, message = %q", err, err)
	})

	t.Run("Get SCIM User", func(t *testing.T) {
		user, err := client.GetUser(ctx, scimUserName)
		require.NoError(t, err)
		require.Equal(t, scimUserName, user.UserName)
		assertSCIMUserSchema(t, user)
		require.Equal(t, scimUserExtraAttrs, user.Attributes)
	})

	t.Run("Update SCIM User", func(t *testing.T) {
		user, err := client.GetUser(ctx, scimUserName)
		require.NoError(t, err)

		// update with no changes
		updatedUser, err := client.UpdateUser(ctx, user)
		require.NoError(t, err)
		require.NotNil(t, updatedUser)
		require.Equal(t, scimUserExtraAttrs, updatedUser.Attributes)

		// attribute addition
		const additionalAttr = "additional_attr"
		updatedUser.Attributes[additionalAttr] = "additional_value"
		extendedAttrs := updatedUser.Attributes
		updatedUser, err = client.UpdateUser(ctx, updatedUser)
		require.NoError(t, err)
		require.NotNil(t, updatedUser)
		require.NotEqual(t, scimUserExtraAttrs, updatedUser.Attributes)
		require.Equal(t, extendedAttrs, updatedUser.Attributes)

		// attribute removal
		delete(updatedUser.Attributes, additionalAttr)
		updatedUser, err = client.UpdateUser(ctx, updatedUser)
		require.NoError(t, err)
		require.NotNil(t, updatedUser)
		require.Equal(t, scimUserExtraAttrs, updatedUser.Attributes)

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
		// Setting `user.Active = false` will not work because of the omitempty json tag.
		user.Attributes = scimsdk.AttributeSet{"active": false}
		_, err = client.UpdateUser(ctx, user)
		require.NoError(t, err)

		// After deactivation the user should delete from downstream system
		// and should not be found.
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			user, err = client.GetUser(ctx, scimUserName)
			var notFound *trace.NotFoundError
			require.ErrorAs(t, err, &notFound, "User %s must have been deleted", scimUserName)
		}, time.Second*3, time.Millisecond*50)
	})

	var group *scimsdk.Group
	var groupName = "Test SCIM Group1"
	t.Run("Create SCIM Group", func(t *testing.T) {
		fakeOkta.CreateOktaGroup(groupName)
		scimGroups := []*scimsdk.Group{{DisplayName: groupName}}
		groups := provisionSCIMGroups(t, client, scimGroups)
		require.Len(t, groups, 1)
		group = groups[0]
		assertSCIMGroupSchema(t, group)
	})

	t.Run("Get SCIM Group", func(t *testing.T) {
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			got, err := client.GetGroupByDisplayName(ctx, groupName)
			require.NoError(t, err)
			require.Equal(t, groupName, got.DisplayName)
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
	t.Parallel()
	ctx := t.Context()

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
		withUserCount(3),
		withAppCount(3),
		withGroupCount(3),
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)

	scimToken := createAndWaitForOktaIntegration(t, sut, fakeOkta)
	scimClient := createSCIMClient(t, sut, scimToken)

	scimUsers := provisionSCIMUsers(t, scimClient, "001", "002", "003")
	testUserDeactivationActivation(t, ctx, sut, scimClient, scimUsers[0].ID)
}

func TestSCIMUserOktaProvisioningWithoutAccessListSyncDisabled(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
		withUserCount(3),
		withAppCount(3),
		withGroupCount(3),
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)

	scimToken := createAndWaitForOktaIntegration(t, sut, fakeOkta, withAccessListDisabled())
	scimClient := createSCIMClient(t, sut, scimToken)

	scimUsers := provisionSCIMUsers(t, scimClient, "001", "002", "003")
	testUserDeactivationActivation(t, ctx, sut, scimClient, scimUsers[0].ID)
}

func TestSCIMOktaGroupProvisioning(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
		withUserCount(3),
		withAppCount(3),
		withGroupCount(3),
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)

	scimToken := createAndWaitForOktaIntegration(t, sut, fakeOkta)
	scimClient := createSCIMClient(t, sut, scimToken)

	scimUsers := provisionSCIMUsers(t, scimClient, "001", "002", "003")

	scimGroup := fakeOkta.CreateOktaGroup("Test SCIM Group1")

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
			require.NoError(t, err)
			require.Len(t, members, 1)
			require.Equal(t, scimUsers[0].ID, members[0].GetName())
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
			require.NoError(t, err)
			require.Len(t, members, 2)
		}, time.Second, time.Millisecond*50)
	})

	t.Run("SCIM group deprovisioning should delete ACL", func(t *testing.T) {
		err := scimClient.DeleteGroup(ctx, scimGroup.Id)
		require.NoError(t, err)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			_, err = sut.Teleport.Process.GetAuthServer().GetAccessList(ctx, scimGroup.Id)
			require.True(t, trace.IsNotFound(err))
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
		require.NoError(t, err)
		require.Len(t, userLocks, 1)

		require.Equal(t, types.OriginOkta, userLocks[0].Origin())
		require.Equal(t, libokta.LockReasonDeactivated, userLocks[0].GetAllLabels()[teleport.OktaLockReasonLabel])
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
		require.NoError(t, err)
		require.Empty(t, userLocks)
	}, time.Second, time.Millisecond*40)
}
