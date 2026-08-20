package okta

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/clientutils"
	libokta "github.com/gravitational/teleport/e/lib/okta"
	oktacommon "github.com/gravitational/teleport/e/lib/okta/common"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/lib/itertools/stream"
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

	testSCIMCRUD(t, sut, fakeOkta, createSCIMClient(t, sut, validToken))
}

func testSCIMCRUD(t *testing.T, sut *common.SUT, fakeOkta *fakeOktaServer, client scimsdk.Client) {
	ctx := t.Context()
	scimUserName := "test-user+001@example.com"
	scimUserExternalID := "test-user-001"
	startingAttrs := scimsdk.AttributeSet{
		"arbitrary_attr_name":   []any{"value"},
		"arbitrary_attr_number": float64(8),
	}
	scimUser := &scimsdk.User{ExternalID: scimUserExternalID, UserName: scimUserName, Active: true, Attributes: startingAttrs}
	userWatcher := sut.NewResourceWatcher(t, types.KindUser)

	t.Run("Create SCIM User", func(t *testing.T) {
		createdUser, err := client.CreateUser(ctx, scimUser)
		require.NoError(t, err)
		require.NotNil(t, createdUser)
		assertSCIMUserSchema(t, createdUser)
		require.Equal(t, startingAttrs, createdUser.Attributes)
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
		require.Equal(t, startingAttrs, user.Attributes)
	})

	t.Run("Update SCIM User", func(t *testing.T) {
		user, err := client.GetUser(ctx, scimUserName)
		require.NoError(t, err)

		// update with no changes
		updatedUser, err := client.UpdateUser(ctx, user)
		require.NoError(t, err)
		require.NotNil(t, updatedUser)
		require.Equal(t, startingAttrs, updatedUser.Attributes)

		// attribute addition
		const additionalAttr = "additional_attr"
		extendedAttrs := copyWithExtraVal(startingAttrs, additionalAttr, "additional_value")

		updatedUser.Attributes = extendedAttrs
		updatedUser, err = client.UpdateUser(ctx, updatedUser)
		require.NoError(t, err)
		require.NotNil(t, updatedUser)
		require.NotEqual(t, startingAttrs, updatedUser.Attributes)
		require.Equal(t, extendedAttrs, map[string]any(updatedUser.Attributes))

		// attribute removal
		delete(updatedUser.Attributes, additionalAttr)
		updatedUser, err = client.UpdateUser(ctx, updatedUser)
		require.NoError(t, err)
		require.NotNil(t, updatedUser)
		require.Equal(t, startingAttrs, updatedUser.Attributes)

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
		common.WaitForDeleteEvent(t, userWatcher, func(r types.Resource) bool {
			return r.GetName() == scimUserName
		})
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

// TestJITSAMLUserProvisioning verifies that JIT SAML users which don't have Okta user ID set, can
// be provisioned. During the first SCIM PUT request Okta doesn't sent the "externalId" attribute
// in the request.
func TestJITSAMLUserProvisioning(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
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

	// Create a user as it was created by the SAML connector.
	const aliceOktaName = "alice@okta.test"
	alice := mustNewSAMLLikeUser(t, aliceOktaName, idp.TestOktaSAMLConnectorName)
	alice, err := sut.Teleport.Process.GetAuthServer().CreateUser(ctx, alice)
	require.NoError(t, err)

	// The JIT SAML connector doesn't have Okta ID set.
	_, ok := alice.GetStaticLabels()[eteleport.OktaUserIDLabel]
	require.False(t, ok)

	// This is an equivalent of Okta PUT request like:
	// NOTE: There is no "externalId" attribute in this request and we need to handle it.
	// {"id":"alice@okta.test","meta":{"created":"2026-02-13T13:36:18.443895Z","location":"/Users/alice@okta.test","resourceType":"User","version":"W/\"09954edc-f927-4c16-9750-99e045fdb225\""},"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"alice@okta.test","active":true}
	// Such request comes right after clicking **Provision User** after the SCIM integration is set up.
	_, err = scimClient.UpdateUser(ctx, &scimsdk.User{
		ID: alice.GetName(),
		Meta: &scimsdk.Metadata{
			Version: `W/"` + alice.GetRevision() + `"`,
		},
		UserName: alice.GetName(),
		Active:   true,
	})
	require.NoError(t, err)
}

// TestSyncedUserProvisioning verifies it is possible to update freshly Okta-synced user with SCIM.
func TestSyncedUserProvisioning(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
		withUserCount(3),
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	for _, user := range fakeOkta.provisionedUsers {
		require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, user.Id))
	}

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)

	authServer := sut.Teleport.Process.GetAuthServer()

	startTime := time.Now()
	scimToken := createAndWaitForOktaIntegration(t, sut, fakeOkta, withEnableFullSync())
	scimClient := createSCIMClient(t, sut, scimToken)

	waitForOktaSync(t, sut, withTimePoint(startTime))

	users := mustListOktaUsers(t, authServer.Services)
	require.Len(t, users, 3)

	// Take a random Okta-synced user from the backend and update it with SCIM client.
	user1 := users[1]

	// Memorize the okta-user-id label value.
	user1ID := mustGetUserIDLabelValue(t, user1)

	// This is an equivalent of Okta PUT request like:
	// NOTE: There is no "externalId" attribute in this request and we need to handle it.
	// {"id":"alice@okta.test","meta":{"created":"2026-02-13T13:36:18.443895Z","location":"/Users/alice@okta.test","resourceType":"User","version":"W/\"09954edc-f927-4c16-9750-99e045fdb225\""},"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"alice@okta.test","active":true}
	// Such request comes right after clicking **Provision User** after the SCIM integration is set up.
	scimUpdatedUser, err := scimClient.UpdateUser(ctx, &scimsdk.User{
		ID: user1.GetName(),
		Meta: &scimsdk.Metadata{
			Version: `W/"` + user1.GetRevision() + `"`,
		},
		UserName: user1.GetName(),
		Active:   true,
	})
	require.NoError(t, err)
	require.Equal(t, user1ID, scimUpdatedUser.ExternalID)
	require.NotEqual(t, user1.GetRevision(), scimUpdatedUser.Meta.Version)

	backendUpdatedUser := mustGetUser(t, authServer, user1.GetName())
	require.Equal(t, user1ID, mustGetUserIDLabelValue(t, backendUpdatedUser))
}

// TestUserProvisioningNoExternalID tests SCIM user creation when the externalId attribute (okta
// user ID) is not provided with the request.
func TestUserProvisioningNoExternalID(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)

	authServer := sut.Teleport.Process.GetAuthServer()

	scimToken := createAndWaitForOktaIntegration(t, sut, fakeOkta)
	scimClient := createSCIMClient(t, sut, scimToken)

	const alice, bob, bobOktaID = "alice", "bob", "bob_okta_id"

	requireUserNotExists(t, authServer, alice)
	requireUserNotExists(t, authServer, bob)

	// alice with no externalId
	_, err := scimClient.CreateUser(ctx, &scimsdk.User{
		ID:       alice,
		Meta:     &scimsdk.Metadata{},
		UserName: alice,
		Active:   true,
	})
	require.NoError(t, err)

	// bob with an externalId
	_, err = scimClient.CreateUser(ctx, &scimsdk.User{
		ID:         bob,
		ExternalID: bobOktaID,
		Meta:       &scimsdk.Metadata{},
		UserName:   bob,
		Active:     true,
	})
	require.NoError(t, err)

	aliceUser := mustGetUser(t, authServer, alice)
	bobUser := mustGetUser(t, authServer, bob)

	require.Empty(t, aliceUser.GetStaticLabels()[eteleport.OktaUserIDLabel])
	require.Equal(t, bobOktaID, bobUser.GetStaticLabels()[eteleport.OktaUserIDLabel])
}

func TestNonProvisionedUsersAreMarkedActive(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Setup Okta mock.
	fakeOkta := newFakeOktaServer(
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

	// Create a user as it was created by the SAML connector.
	const aliceOktaName = "alice@okta.test"
	alice := mustNewSAMLLikeUser(t, aliceOktaName, idp.TestOktaSAMLConnectorName)
	_, err := sut.Teleport.Process.GetAuthServer().CreateUser(ctx, alice)
	require.NoError(t, err)

	// Make sure the user's "active" attribute is set to true even if the user was not
	// provisioned yet. This is so future PUT requests can work and not interpret the user as
	// inactive, resulting in error like:
	//
	//	Automatic provisioning of user Alice to app Teleport failed: User account is inactive
	//
	aliceSCIM, err := scimClient.GetUser(ctx, aliceOktaName)
	require.NoError(t, err)
	require.True(t, aliceSCIM.Active)
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

	memberWatcher := sut.NewResourceWatcher(t, types.KindAccessListMember)
	accessListWatcher := sut.NewResourceWatcher(t, types.KindAccessList)
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

		member := waitForResource(t, memberWatcher, func(m *accesslist.AccessListMember) bool {
			return m.Spec.AccessList == scimGroup.Id
		})
		require.Equal(t, scimUsers[0].ID, member.GetName())
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

		waitForResource(t, memberWatcher, func(m *accesslist.AccessListMember) bool {
			return m.Spec.AccessList == scimGroup.Id && m.GetName() == scimUsers[1].ID
		})
		members := mustListAccessListMembers(t, sut, scimGroup.Id)
		require.Len(t, members, 2)
	})

	t.Run("SCIM group deprovisioning should delete ACL", func(t *testing.T) {
		err := scimClient.DeleteGroup(ctx, scimGroup.Id)
		require.NoError(t, err)

		waitForResourceDeletion(t, accessListWatcher, func(rh *types.ResourceHeader) bool {
			return rh.GetName() == scimGroup.Id
		})

		_, err = sut.Teleport.Process.GetAuthServer().GetAccessList(ctx, scimGroup.Id)
		require.True(t, trace.IsNotFound(err))
	})
}

func testUserDeactivationActivation(t *testing.T, ctx context.Context, sut *common.SUT, client scimsdk.Client, username string) {
	auth := sut.Teleport.Process.GetAuthServer()
	user, err := client.GetUserByUserName(ctx, username)
	require.NoError(t, err)

	lockWatcher := sut.NewResourceWatcher(t, types.KindLock)
	userWatcher := sut.NewResourceWatcher(t, types.KindUser)
	lockTarget := types.LockTarget{User: user.ID}

	user.Attributes = scimsdk.AttributeSet{"active": false}
	_, err = client.UpdateUser(ctx, user)
	require.NoError(t, err)

	// When user is deactivated the user should be removed from teleport backend
	// and a user lock should be created to kill all teleport active sessions.
	lock := waitForResource(t, lockWatcher, lockTarget.Match)
	require.Equal(t, types.OriginOkta, lock.Origin())
	require.Equal(t, libokta.LockReasonDeactivated, lock.GetAllLabels()[eteleport.OktaLockReasonLabel])

	waitForResourceDeletion(t, userWatcher, func(rh *types.ResourceHeader) bool {
		return rh.GetName() == user.ID
	})
	_, err = auth.GetUser(ctx, user.ID, false)
	require.True(t, trace.IsNotFound(err))

	// Recover user by settings the user to active state. And recreate the user.
	// User recreation is peculiar Okta behavior, when user is recovered from deactivated state  Okta will
	//  create a new user instead of updating the existing user.
	user.Attributes = scimsdk.AttributeSet{"active": true}
	_, err = client.CreateUser(ctx, user)
	require.NoError(t, err)

	_, err = auth.GetUser(ctx, user.ID, false)
	require.NoError(t, err)

	// If a user was re-activated, the lock created by deactivation flow should be removed.
	waitForResourceDeletion(t, lockWatcher, func(rh *types.ResourceHeader) bool {
		return rh.GetName() == lock.GetName()
	})
	userLocks, err := auth.GetLocks(ctx, false, lockTarget)
	require.NoError(t, err)
	require.Empty(t, userLocks)
}

// TestSCIMOnlyAPICredentials verifies that SCIM user provisioning correctly populates user's
// "groups" trait depending on the state of the Okta API credential in the backend. In particular
// it verifies the behavior with the legacy SCIM-only Okta API token. See
// [mustConvertToOktaSCIMOnlyAPICredential] godoc for more details.
func TestSCIMOnlyAPICredentials(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	fakeOkta := newFakeOktaServer(
		withUserCount(4),
		withAppCount(4),
		withGroupCount(4),
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)

	authServer := sut.Teleport.Process.GetAuthServer()

	oktaCreds := oktav1.OktaAPICredentials_builder{SswsBearerToken: proto.String("12345")}.Build()
	scimToken := createAndWaitForOktaIntegration(t, sut, fakeOkta, withAPICredentials(oktaCreds), withAccessListDisabled())
	scimClient := createSCIMClient(t, sut, scimToken)

	u1, u2, u3 := fakeOkta.provisionedUsers[1], fakeOkta.provisionedUsers[2], fakeOkta.provisionedUsers[3]
	g1, g2 := fakeOkta.provisionedGroups[1], fakeOkta.provisionedGroups[2]

	fakeOkta.AddUserToGroup(g1.Id, u1.Id)
	fakeOkta.AddUserToGroup(g2.Id, u1.Id)
	fakeOkta.AddUserToGroup(g1.Id, u2.Id)
	fakeOkta.AddUserToGroup(g2.Id, u2.Id)
	fakeOkta.AddUserToGroup(g1.Id, u3.Id)
	fakeOkta.AddUserToGroup(g2.Id, u3.Id)

	groupTraits := map[string][]string{"groups": {g1.Profile.Name, g2.Profile.Name}}

	requireUserNotExists(t, authServer, oktaUserLogin(u1))
	requireUserNotExists(t, authServer, oktaUserLogin(u2))

	// Create user with the default API credentials, and expect it to have groups trait set
	// because the credential is present in the backend.
	_, err := scimClient.CreateUser(ctx, &scimsdk.User{ExternalID: u1.Id, UserName: oktaUserLogin(u1), Active: true})
	require.NoError(t, err)

	teleportUser1 := mustGetUser(t, authServer, oktaUserLogin(u1))
	requireTraitsEqual(t, groupTraits, teleportUser1.GetTraits())
	requireUserNotExists(t, authServer, oktaUserLogin(u2))

	// Convert to the legacy SCIM-only API token.
	// Then, Create user with the legacy SCIM-only API, and expect it to have groups trait set
	// because the credential is present in the backend.
	mustConvertToOktaSCIMOnlyAPICredential(t, sut)

	_, err = scimClient.CreateUser(ctx, &scimsdk.User{ExternalID: u2.Id, UserName: oktaUserLogin(u2), Active: true})
	require.NoError(t, err)

	teleportUser2 := mustGetUser(t, authServer, oktaUserLogin(u2))
	requireTraitsEqual(t, groupTraits, teleportUser2.GetTraits())

	// Delete the API token and create a third user with SCIM, but now the groups trait
	// shouldn't be set.
	mustDeleteOktaAPICredential(t, sut)

	_, err = scimClient.CreateUser(ctx, &scimsdk.User{ExternalID: u3.Id, UserName: oktaUserLogin(u3), Active: true})
	require.NoError(t, err)

	teleportUser3 := mustGetUser(t, authServer, oktaUserLogin(u3))
	require.Empty(t, teleportUser3.GetTraits())
}

// TestSCIMGroupUpdateWithPendingAssignments verifies that SCIM push
// propagates group membership changes even when bidirectional sync
// (the Teleport -> Okta assignment processor) is disabled.
//
// When bidirectional sync is disabled, the flow still crates
// OktaAssignment records. However, because the assignment processor loop is not running.
// The SCIM push should not filter out the group members that have PENDING OktaAssignment records.
func TestSCIMGroupUpdateWithPendingAssignments(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	fakeOkta := newFakeOktaServer(
		withUserCount(3),
		withAppCount(1),
		withGroupCount(1),
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	// Assign all users to the SAML app so the sync can import them.
	for _, u := range fakeOkta.provisionedUsers {
		require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, u.Id))
		require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedApps[0].Id, u.Id))
	}

	groupID := fakeOkta.provisionedGroups[0].Id

	// Put all three users in the Okta group so the access-list sync picks them
	// up as group members and creates per-user PENDING OktaAssignment records.
	for _, u := range fakeOkta.provisionedUsers[0:2] {
		fakeOkta.AddUserToGroup(groupID, u.Id)
	}

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)

	assignmentWatcher := sut.NewResourceWatcher(t, types.KindOktaAssignment)

	// Use access-list sync without bidirectional sync. The Okta -> Teleport import
	// runs and creates PENDING OktaAssignment records, but the assignment
	// processor (Teleport -> Okta) is disabled, so the assignments stay PENDING.
	scimToken := createAndWaitForOktaIntegration(t, sut, fakeOkta,
		withAccessListSettings(oktav1.AccessListSettings_builder{
			GroupFilters: []string{"group-*"},
			AppFilters:   []string{},
			DefaultOwner: []string{"alice-admin"},
		}.Build()),
		withAccessListSyncEnabledNoBidirectional(),
		withTimeBetweenImports(time.Hour),
	)
	scimClient := createSCIMClient(t, sut, scimToken)
	authServer := sut.Teleport.Process.GetAuthServer()

	// Wait for the access-list sync to create the per-user PENDING assignments.
	waitForPerUserOktaAssignments(t, assignmentWatcher, len(fakeOkta.provisionedUsers))

	syncedUsers := mustListOktaUsers(t, authServer.Services)
	require.Len(t, syncedUsers, len(fakeOkta.provisionedUsers))

	group, err := scimClient.GetGroup(ctx, groupID)
	require.NoError(t, err)
	require.Len(t, group.Members, 2)
	require.NotEqual(t, len(syncedUsers), len(group.Members))

	group.Members = make([]*scimsdk.GroupMember, len(syncedUsers))
	for i, u := range syncedUsers {
		group.Members[i] = &scimsdk.GroupMember{ExternalID: u.GetName()}
	}
	updatedGroup, err := scimClient.UpdateGroup(ctx, group)
	require.NoError(t, err)
	require.Len(t, updatedGroup.Members, len(fakeOkta.provisionedUsers),
		"regression: pendingAssignmentFilter stripped all members from SCIM group push")

	// TimeBetweenImports is set to 1h, so we can be sure here AccessList sync won't compete to
	// revert SCIM push changes here.
	members, err := stream.Collect(clientutils.Resources(ctx,
		func(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
			return authServer.Services.ListAccessListMembers(ctx, groupID, pageSize, pageToken)
		},
	))
	require.NoError(t, err)
	require.Len(t, members, len(fakeOkta.provisionedUsers))
}
