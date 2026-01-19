package scim

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	typescommon "github.com/gravitational/teleport/api/types/common"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/lib/defaults"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
)

func getSCIMUserName(u *scimsdk.User) string {
	return u.UserName
}

func TestSCIMGeneric(t *testing.T) {
	t.Parallel()

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)
	authClient := sut.Teleport.Process.GetAuthServer()
	aclClient := authClient.AccessListsInternal

	scimToken := createGenericSCIMPlugin(t, sut)
	scimClient := createPluginSCIMClient(t, sut, scimToken, "generic")

	scimUser1 := newSCIMUser("scim-user-001")
	scimUser2 := newSCIMUser("scim-user-002")

	for _, user := range []*scimsdk.User{scimUser1, scimUser2} {
		createdUser, err := scimClient.CreateUser(t.Context(), user)
		require.NoError(t, err)
		require.Equal(t, user.UserName, createdUser.UserName)

		retrievedUser, err := scimClient.GetUser(t.Context(), createdUser.ID)
		require.NoError(t, err)
		require.Equal(t, createdUser.ID, retrievedUser.ID)

		u, err := authClient.GetUser(t.Context(), createdUser.UserName, false)
		require.NoError(t, err)

		require.Equal(t, &types.ConnectorRef{
			ID:       "okta-pre-created-test",
			Type:     "saml",
			Identity: user.ExternalID,
		}, u.GetCreatedBy().Connector)
	}
	scimListUsersResp, err := scimClient.ListUsers(t.Context())
	require.NoError(t, err)
	require.Equal(t, int32(2), scimListUsersResp.TotalResults)

	scimGroup := &scimsdk.Group{DisplayName: "test-group-001"}

	// Reject group creation before access list exists"
	_, err = scimClient.CreateGroup(t.Context(), scimGroup)
	require.Error(t, err)

	common.CreateAccessList(t, sut,
		common.WithName("test-group-001"),
		common.WithAccessListType(accesslist.SCIM),
		common.WithOwners("alice-admin"),
		common.WithGrants(accesslist.Grants{Roles: []string{"access"}}),
	)

	// Create group after access list exists
	createdGroup, err := scimClient.CreateGroup(t.Context(), scimGroup)
	require.NoError(t, err)

	acls, err := aclClient.GetAccessLists(t.Context())
	require.NoError(t, err)
	require.Len(t, acls, 1)

	t.Run("Add members to group", func(t *testing.T) {
		users := []*scimsdk.User{scimUser1, scimUser2}

		for i, user := range users {
			t.Run(strconv.Itoa(i+1), func(t *testing.T) {
				group, err := scimClient.GetGroup(t.Context(), createdGroup.ID)
				require.NoError(t, err)

				group.Members = append(group.Members, &scimsdk.GroupMember{
					ExternalID: user.UserName,
				})
				group, err = scimClient.UpdateGroup(t.Context(), group)
				require.NoError(t, err)

				accessListMembers := common.GetAccessListMembers(t, sut, group.ID)
				require.Len(t, accessListMembers, i+1)
				require.ElementsMatch(t,
					sliceutils.Map(accessListMembers, common.GetAccessListMemberName),
					sliceutils.Map(users[:i+1], getSCIMUserName))

				for _, m := range accessListMembers {
					require.Equal(t, "SCIM", m.Spec.AddedBy)
				}
			})
		}
	})

	t.Run("Listing and getting users should include groups", func(t *testing.T) {
		expectedGroups := []any{
			map[string]any{"value": "test-group-001"},
		}

		listResp, err := scimClient.ListUsers(t.Context())
		require.NoError(t, err)
		require.Len(t, listResp.Users, 2)
		for _, u := range listResp.Users {
			require.Equal(t, expectedGroups, u.Attributes["groups"], "user ID = %q", u.ID)
		}

		u1, err := scimClient.GetUser(t.Context(), scimUser1.UserName)
		require.NoError(t, err)
		require.Equal(t, expectedGroups, u1.Attributes["groups"])

		u2, err := scimClient.GetUser(t.Context(), scimUser2.UserName)
		require.NoError(t, err)
		require.Equal(t, expectedGroups, u2.Attributes["groups"])
	})

	t.Run("Remove first member from group", func(t *testing.T) {
		group, err := scimClient.GetGroup(t.Context(), createdGroup.ID)
		require.NoError(t, err)
		group.Members = []*scimsdk.GroupMember{
			{ExternalID: scimUser2.UserName},
		}
		_, err = scimClient.UpdateGroup(t.Context(), group)
		require.NoError(t, err)

		members, _, err := aclClient.ListAccessListMembers(t.Context(), createdGroup.ID, 0, "")
		require.NoError(t, err)
		require.Len(t, members, 1)
		require.Equal(t, scimUser2.UserName, members[0].GetName())
		require.Equal(t, "SCIM", members[0].Spec.AddedBy)
	})

	t.Run("Delete group", func(t *testing.T) {
		err := scimClient.DeleteGroup(t.Context(), createdGroup.ID)
		require.NoError(t, err)

		members, _, err := aclClient.ListAccessListMembers(t.Context(), createdGroup.ID, 0, "")
		require.NoError(t, err)
		require.Empty(t, members)
	})

	err = scimClient.DeleteUser(t.Context(), scimUser1.UserName)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		_, err = authClient.GetUser(context.Background(), scimUser1.UserName, false)
		require.Error(t, err)
	}, time.Second, 20*time.Millisecond)

	t.Run("upgrade SSO ephemeral user to SCIM user", func(t *testing.T) {
		fistUserName := "user-001@exmaple.com"
		secondUserName := "user-002example.com"
		thirdUserName := "user-003example.com"
		u1 := newTeleportUser(t, fistUserName, types.ConnectorRef{ID: "okta-pre-created-test", Type: types.KindSAML})
		u2 := newTeleportUser(t, secondUserName, types.ConnectorRef{ID: "no-scim-plugin-connector", Type: types.KindSAML})
		// Test connector type logic where connector nane is not unique across SAML and OIDC connector.
		// Depending on user origin SAML or OIDC ephemeral users upgrade should succeed or fail.
		u3 := newTeleportUser(t, thirdUserName, types.ConnectorRef{ID: "okta-pre-created-test", Type: types.KindOIDC})

		for _, u := range []types.User{u1, u2, u3} {
			_, err := authClient.CreateUser(context.Background(), u)
			require.NoError(t, err)

			// Wait for user to be propagated to the cache.
			require.EventuallyWithT(t, func(t *assert.CollectT) {
				_, err := authClient.GetUser(context.Background(), u.GetName(), false)
				require.NoError(t, err)
			}, time.Second, 30*time.Millisecond)
		}
		_, err := scimClient.CreateUser(context.Background(), &scimsdk.User{
			ExternalID: fistUserName,
			UserName:   fistUserName,
			Active:     true,
		})
		require.NoError(t, err)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			u, err := authClient.GetUser(context.Background(), fistUserName, false)
			require.NoError(t, err)
			require.Equal(t, typescommon.OriginSCIM, u.Origin())
		}, time.Second, time.Millisecond*30)

		// Attempt to create an SCIM user with a name that already exists in Teleport
		// but is not managed by SCIM connector. This should fail.
		_, err = scimClient.CreateUser(context.Background(), &scimsdk.User{
			ExternalID: secondUserName,
			UserName:   secondUserName,
			Active:     true,
		})
		require.Error(t, err)

		// Attempt to create an SCIM users that duplicates with the existing user
		// that are handled by OIDC connector where the SCIM plugin was configured
		// with the SAML connector.
		_, err = scimClient.CreateUser(context.Background(), &scimsdk.User{
			ExternalID: thirdUserName,
			UserName:   thirdUserName,
			Active:     true,
		})
		require.Error(t, err)
	})

	t.Run("test scim rate limiting", func(t *testing.T) {
		requestCount := defaults.LimiterBurst + defaults.LimiterAverage
		var errGroup errgroup.Group

		errGroup.SetLimit(10)

		// SCIM Client with proper token should not be rate limited.
		for i := 0; i < requestCount+1; i++ {
			errGroup.Go(func() error {
				_, err := scimClient.ListUsers(t.Context())
				return err
			})
		}
		require.NoError(t, errGroup.Wait())
	})

}

func TestOIDCConnector(t *testing.T) {
	t.Parallel()

	sut := common.InitSUT(t,
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithResources(createOIDConnector(t, "scim-oidc-connector")),
	)

	scimToken := createGenericSCIMPlugin(t, sut,
		withSCIMSettings(&types.PluginSCIMSettings{
			ConnectorInfo: &types.PluginSCIMSettings_ConnectorInfo{
				Name: "scim-oidc-connector",
				Type: types.KindOIDC,
			},
		}),
	)
	authClient := sut.Teleport.Process.GetAuthServer()
	scimClient := createPluginSCIMClient(t, sut, scimToken, "generic")

	scimUser1 := newSCIMUser("scim-user-001")
	scimUser2 := newSCIMUser("scim-user-002")

	for _, user := range []*scimsdk.User{scimUser1, scimUser2} {
		createdUser, err := scimClient.CreateUser(t.Context(), user)
		require.NoError(t, err)
		require.Equal(t, user.UserName, createdUser.UserName)

		retrievedUser, err := scimClient.GetUser(t.Context(), createdUser.ID)
		require.NoError(t, err)
		require.Equal(t, createdUser.ID, retrievedUser.ID)

		u, err := authClient.GetUser(t.Context(), createdUser.UserName, false)
		require.NoError(t, err)

		require.Equal(t, &types.ConnectorRef{
			ID:       "scim-oidc-connector",
			Type:     types.KindOIDC,
			Identity: user.ExternalID,
		}, u.GetCreatedBy().Connector)
	}
}

func copy[T any](value *T) *T {
	copy := *value
	return &copy
}

func asMember(user *scimsdk.User) *scimsdk.GroupMember {
	return &scimsdk.GroupMember{ExternalID: user.ID}
}

// TestOverlappedGroupUpdates asserts that updates based on an old revision of a
// Group are considered and error.
func TestOverlappedGroupUpdates(t *testing.T) {
	t.Parallel()

	const groupName = "test-group-001"

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)
	scimToken := createGenericSCIMPlugin(t, sut)
	scimClient := createPluginSCIMClient(t, sut, scimToken, "generic")

	// GIVEN some users in the cluster...
	var users []*scimsdk.User
	for i := range 10 {
		user, err := scimClient.CreateUser(t.Context(), newSCIMUser(fmt.Sprintf("scim-user-%03d", i)))
		require.NoError(t, err)
		users = append(users, user)
	}

	// GIVEN an Access List...
	common.CreateAccessList(t, sut,
		common.WithName(groupName),
		common.WithAccessListType(accesslist.SCIM),
		common.WithOwners("alice-admin"),
		common.WithGrants(accesslist.Grants{Roles: []string{"access"}}),
	)
	originalGroup, err := scimClient.GetGroup(t.Context(), groupName)
	require.NoError(t, err)

	// GIVEN a SCIM Group update that will change the underlying resource's
	// version string
	updatedGroup := copy(originalGroup)
	updatedGroup.Members = sliceutils.Map(users, asMember)
	updatedGroup, err = scimClient.UpdateGroup(t.Context(), updatedGroup)
	require.NoError(t, err)
	require.NotEqual(t, updatedGroup.Meta.Version, originalGroup.Meta.Version,
		"Expected resource version to change")

	// WHEN I attempt to update the Access List based on the original, unmodified
	// group...
	overlappedGroup := copy(originalGroup)
	overlappedGroup.Members = sliceutils.Map(users[2:], asMember)
	_, err = scimClient.UpdateGroup(t.Context(), overlappedGroup)

	// EXPECT the update to fail
	var conflictErr *trace.CompareFailedError
	require.ErrorAs(t, err, &conflictErr)

	// EXPECT that the remote group members still match the first update
	finalGroup, err := scimClient.GetGroup(t.Context(), groupName)
	require.NoError(t, err)
	require.ElementsMatch(t, updatedGroup.Members, finalGroup.Members)
}

// TestUnversionedOverlappedGroupUpdates asserts that group updates with no
// version specified still succeed. This behavior is for backwards
// compatibility with previous releases of Teleport.
func TestUnversionedOverlappedGroupUpdates(t *testing.T) {
	t.Parallel()

	const groupName = "test-group-001"

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)
	scimToken := createGenericSCIMPlugin(t, sut)
	scimClient := createPluginSCIMClient(t, sut, scimToken, "generic")

	// GIVEN some users in the cluster...
	var users []*scimsdk.User
	for i := range 10 {
		user, err := scimClient.CreateUser(t.Context(), newSCIMUser(fmt.Sprintf("scim-user-%03d", i)))
		require.NoError(t, err)
		users = append(users, user)
	}

	// GIVEN an Access List...
	common.CreateAccessList(t, sut,
		common.WithName(groupName),
		common.WithAccessListType(accesslist.SCIM),
		common.WithOwners("alice-admin"),
		common.WithGrants(accesslist.Grants{Roles: []string{"access"}}),
	)
	originalGroup, err := scimClient.GetGroup(t.Context(), groupName)
	require.NoError(t, err)

	// GIVEN a SCIM Group update that will change the underlying resource's
	// version string
	updatedGroup := copy(originalGroup)
	updatedGroup.Members = sliceutils.Map(users, asMember)
	updatedGroup, err = scimClient.UpdateGroup(t.Context(), updatedGroup)
	require.NoError(t, err)
	require.NotEqual(t, updatedGroup.Meta.Version, originalGroup.Meta.Version,
		"Expected resource version to change")

	// WHEN I attempt to update the Access List based on the original, unmodified
	// group, WITHOUT specifying a base revision...
	overlappedGroup := copy(originalGroup)
	overlappedGroup.Meta.Version = ""
	overlappedGroup.Members = sliceutils.Map(users[2:], asMember)
	overlappedGroup, err = scimClient.UpdateGroup(t.Context(), overlappedGroup)

	// EXPECT the update to succeed and that the remote group members have been
	// updated
	require.NoError(t, err)
	finalGroup, err := scimClient.GetGroup(t.Context(), groupName)
	require.NoError(t, err)
	require.ElementsMatch(t, overlappedGroup.Members, finalGroup.Members)
}

// TestOverlappedGroupUpdates asserts that updates based on an old revision of a
// User are considered and error.
func TestOverlappedUserUpdates(t *testing.T) {
	t.Parallel()

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)
	scimToken := createGenericSCIMPlugin(t, sut)
	scimClient := createPluginSCIMClient(t, sut, scimToken, "generic")

	// GIVEN a SCIM-created user in the cluster
	originalUser, err := scimClient.CreateUser(t.Context(), newSCIMUser("scim-user-001"))
	require.NoError(t, err)

	// GIVEN a SCIM User update that will change the underlying resource's
	// version string
	updatedUser := copy(originalUser)
	updatedUser.DisplayName = "Updated!"
	updatedUser, err = scimClient.UpdateUser(t.Context(), updatedUser)
	require.NoError(t, err)
	require.NotEqual(t, updatedUser.Meta.Version, originalUser.Meta.Version,
		"Expected resource version to change")

	// WHEN I attempt to update the User based on the original, unmodified
	// group...
	overlappedUser := copy(originalUser)
	overlappedUser.DisplayName = "Overlapped Update!"
	_, err = scimClient.UpdateUser(t.Context(), overlappedUser)

	// EXPECT the update to fail
	var conflictErr *trace.CompareFailedError
	require.ErrorAs(t, err, &conflictErr)
}

// TestUnversionedOverlappedUserUpdates asserts that user updates with no
// version specified still succeed. This behavior is for backwards
// compatibility with previous releases of Teleport.
func TestUnversionedOverlappedUserUpdates(t *testing.T) {
	t.Parallel()

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)
	scimToken := createGenericSCIMPlugin(t, sut)
	scimClient := createPluginSCIMClient(t, sut, scimToken, "generic")

	// GIVEN a SCIM-created user in the cluster
	originalUser, err := scimClient.CreateUser(t.Context(), newSCIMUser("scim-user-001"))
	require.NoError(t, err)

	// GIVEN a SCIM User update that will change the underlying resource's
	// version string
	updatedUser := copy(originalUser)
	updatedUser.DisplayName = "Updated!"
	updatedUser, err = scimClient.UpdateUser(t.Context(), updatedUser)
	require.NoError(t, err)
	require.NotEqual(t, updatedUser.Meta.Version, originalUser.Meta.Version,
		"Expected resource version to change")

	// WHEN I attempt to update the User based on the original, unmodified
	// group...
	overlappedUser := copy(originalUser)
	overlappedUser.Meta.Version = ""
	overlappedUser.DisplayName = "Overlapped Update!"
	_, err = scimClient.UpdateUser(t.Context(), overlappedUser)

	// EXPECT the update to succeed
	require.NoError(t, err)
}

func newTeleportUser(t *testing.T, userName string, connRef types.ConnectorRef) types.User {
	user, err := types.NewUser(userName)
	require.NoError(t, err)
	user.SetCreatedBy(types.CreatedBy{
		Connector: &connRef,
	})
	return user
}

func newSCIMUser(username string) *scimsdk.User {
	return &scimsdk.User{
		ExternalID: username,
		UserName:   username + "@example.com",
		Active:     true,
	}
}
