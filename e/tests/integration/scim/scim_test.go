package scim

import (
	"context"
	"encoding/json"
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
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
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
	userWatcher := sut.NewResourceWatcher(t, types.KindUser)

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

	common.WaitForDeleteEvent(t, userWatcher, func(r types.Resource) bool {
		return r.GetName() == scimUser1.UserName
	})

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
			common.WaitForPutEvent(t, userWatcher, func(user types.User) bool {
				return user.GetName() == u.GetName()
			})
		}
		_, err := scimClient.CreateUser(context.Background(), &scimsdk.User{
			ExternalID: fistUserName,
			UserName:   fistUserName,
			Active:     true,
		})
		require.NoError(t, err)

		common.WaitForPutEvent(t, userWatcher, func(u types.User) bool {
			return u.GetName() == fistUserName && u.Origin() == typescommon.OriginSCIM
		})

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

// TestPagination verifies that the SCIM ListGroups and ListUsers
// endpoints return a totalResults value that reflects the total number of
// matching resources, independent of the requested page size and start index.
//
// RFC https://datatracker.ietf.org/doc/html/rfc7644#section-3.4.2.4
func TestPagination(t *testing.T) {
	t.Parallel()

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)
	scimToken := createGenericSCIMPlugin(t, sut)
	scimClient := createPluginSCIMClient(t, sut, scimToken, "generic")

	t.Run("Groups", func(t *testing.T) {
		// Create 5 SCIM-typed access lists that will appear as SCIM groups.
		for i := range 5 {
			common.CreateAccessList(t, sut,
				common.WithName(fmt.Sprintf("group-%03d", i+1)),
				common.WithTitle(fmt.Sprintf("Group %03d", i+1)),
				common.WithAccessListType(accesslist.SCIM),
				common.WithOwners("alice-admin"),
			)
		}

		ctx := t.Context()
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			// Verify all 5 groups are visible.
			allGroups, err := scimClient.ListGroups(ctx)
			require.NoError(t, err)
			require.Equal(t, int32(5), allGroups.TotalResults)
			require.Len(t, allGroups.Groups, 5)
		}, time.Minute, time.Millisecond*30)

		t.Run("first page", func(t *testing.T) {
			resp, err := scimClient.ListGroups(t.Context(),
				scimsdk.WithStartIndex(1),
				scimsdk.WithCount(2),
			)
			require.NoError(t, err)
			require.Equal(t, int32(5), resp.TotalResults)
			require.Len(t, resp.Groups, 2)
			require.Equal(t, int32(2), resp.ItemsPerPage)
			require.Equal(t, int32(1), resp.StartIndex)
		})

		t.Run("second page", func(t *testing.T) {
			resp, err := scimClient.ListGroups(t.Context(),
				scimsdk.WithStartIndex(3),
				scimsdk.WithCount(2),
			)
			require.NoError(t, err)
			require.Equal(t, int32(5), resp.TotalResults)
			require.Len(t, resp.Groups, 2)
			require.Equal(t, int32(2), resp.ItemsPerPage)
			require.Equal(t, int32(3), resp.StartIndex)
		})

		t.Run("last page with partial results", func(t *testing.T) {
			resp, err := scimClient.ListGroups(t.Context(),
				scimsdk.WithStartIndex(5),
				scimsdk.WithCount(2),
			)
			require.NoError(t, err)
			require.Equal(t, int32(5), resp.TotalResults)
			require.Len(t, resp.Groups, 1)
			require.Equal(t, int32(1), resp.ItemsPerPage)
		})

		t.Run("past the end", func(t *testing.T) {
			resp, err := scimClient.ListGroups(t.Context(),
				scimsdk.WithStartIndex(6),
				scimsdk.WithCount(2),
			)
			require.NoError(t, err)
			require.Equal(t, int32(5), resp.TotalResults)
			require.Empty(t, resp.Groups)
			require.Equal(t, int32(0), resp.ItemsPerPage)
		})
	})

	t.Run("Users", func(t *testing.T) {
		for i := range 5 {
			_, err := scimClient.CreateUser(t.Context(), newSCIMUser(fmt.Sprintf("user-%03d", i+1)))
			require.NoError(t, err)
		}

		ctx := t.Context()
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			// Verify all 5 users are visible.
			allUsers, err := scimClient.ListUsers(ctx)
			require.NoError(t, err)
			require.Equal(t, int32(5), allUsers.TotalResults)
			require.Len(t, allUsers.Users, 5)

		}, time.Minute, time.Millisecond*30)

		t.Run("first page", func(t *testing.T) {
			resp, err := scimClient.ListUsers(t.Context(),
				scimsdk.WithStartIndex(1),
				scimsdk.WithCount(2),
			)
			require.NoError(t, err)
			require.Equal(t, int32(5), resp.TotalResults)
			require.Len(t, resp.Users, 2)
			require.Equal(t, int32(2), resp.ItemsPerPage)
			require.Equal(t, int32(1), resp.StartIndex)
		})

		t.Run("second page", func(t *testing.T) {
			resp, err := scimClient.ListUsers(t.Context(),
				scimsdk.WithStartIndex(3),
				scimsdk.WithCount(2),
			)
			require.NoError(t, err)
			require.Equal(t, int32(5), resp.TotalResults)
			require.Len(t, resp.Users, 2)
			require.Equal(t, int32(2), resp.ItemsPerPage)
			require.Equal(t, int32(3), resp.StartIndex)
		})

		t.Run("last page with partial results", func(t *testing.T) {
			resp, err := scimClient.ListUsers(t.Context(),
				scimsdk.WithStartIndex(5),
				scimsdk.WithCount(2),
			)
			require.NoError(t, err)
			require.Equal(t, int32(5), resp.TotalResults)
			require.Len(t, resp.Users, 1)
			require.Equal(t, int32(1), resp.ItemsPerPage)
		})

		t.Run("past the end", func(t *testing.T) {
			resp, err := scimClient.ListUsers(t.Context(),
				scimsdk.WithStartIndex(6),
				scimsdk.WithCount(2),
			)
			require.NoError(t, err)
			require.Equal(t, int32(5), resp.TotalResults)
			require.Empty(t, resp.Users)
			require.Equal(t, int32(0), resp.ItemsPerPage)
		})
	})
}

// TestSCIMUserGroupAttribute verifies that the groups attribute is managed
// by the SCIM Service Provider instead of consuming the groups attribute from
// the request body, as the Teleport SCIM Server Schema marks groups as readOnly.
func TestSCIMUserGroupAttribute(t *testing.T) {
	t.Parallel()

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)

	scimToken := createGenericSCIMPlugin(t, sut)
	baseURL := buildURL(sut.ProxyAddr, "/v1/webapi/scim/generic")
	httpClient := newBearerClient(scimToken)
	scimClient := createPluginSCIMClient(t, sut, scimToken, "generic")
	authServer := sut.Teleport.Process.GetAuthServer()

	t.Run("SCIM user groups attribute should be always dynamically calculated", func(t *testing.T) {
		originalUser, err := scimClient.CreateUser(t.Context(), newSCIMUser("scim-user-001"))
		require.NoError(t, err)

		// Overwrite a user object with manual groups attribute to make sure that
		// it will be ignored and calculated by SCIM Service Provider based on actual
		// access list membership.
		teleportUser, err := authServer.Services.GetUser(t.Context(), originalUser.UserName, false)
		require.NoError(t, err)
		labels := teleportUser.GetAllLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[eteleport.SCIMAttrsLabel] = `{"groups":[{"value":"group-091"}]}`
		teleportUser.SetStaticLabels(labels)

		_, err = authServer.UpdateUser(t.Context(), teleportUser)
		require.NoError(t, err)

		u, err := scimClient.GetUser(t.Context(), originalUser.UserName)
		require.NoError(t, err)
		require.Equal(t, []string{}, userGroupNames(u))

		const groupName1 = "group-001"
		common.CreateAccessList(t, sut,
			common.WithName(groupName1),
			common.WithAccessListType(accesslist.SCIM),
			common.WithOwners("alice-admin"),
			common.WithGrants(accesslist.Grants{Roles: []string{"access"}}),
		)
		mustPatchGroup(t, httpClient, baseURL.String(), groupName1, []map[string]any{
			{
				"op":   "add",
				"path": "members",
				"value": []map[string]any{
					{"value": originalUser.UserName},
				},
			},
		})

		u, err = scimClient.GetUser(t.Context(), originalUser.UserName)
		require.NoError(t, err)
		require.Equal(t, []string{groupName1}, userGroupNames(u))

		mustPatchGroup(t, httpClient, baseURL.String(), groupName1, []map[string]any{
			{
				"op":   "remove",
				"path": "members",
				"value": []map[string]any{
					{"value": originalUser.UserName},
				},
			},
		})

		u, err = scimClient.GetUser(t.Context(), originalUser.UserName)
		require.NoError(t, err)
		require.Equal(t, []string{}, userGroupNames(u))
	})

	t.Run("user groups attributes are readOnly", func(t *testing.T) {
		originalUser, err := scimClient.CreateUser(t.Context(), newSCIMUser("scim-user-002"))
		require.NoError(t, err)

		u, err := scimClient.GetUser(t.Context(), originalUser.UserName)
		require.NoError(t, err)

		// Overwriting "groups" attribute in the user object should be not allowed
		// since groups attribute is readOnly
		u.Attributes["groups"] = []string{"group1", "group2"}
		// where other attributes like custom someAttr should be settable by SCIM client.
		u.Attributes["someAttr"] = "someValue"
		_, err = scimClient.UpdateUser(t.Context(), u)
		require.NoError(t, err)

		u, err = scimClient.GetUser(t.Context(), originalUser.UserName)
		require.NoError(t, err)
		require.Equal(t, []string{}, userGroupNames(u))

		teleportUser, err := authServer.Services.GetUser(t.Context(), originalUser.UserName, false)
		require.NoError(t, err)

		v, ok := teleportUser.GetLabel(eteleport.SCIMAttrsLabel)
		require.True(t, ok)

		got := map[string]any{}
		require.NoError(t, json.Unmarshal([]byte(v), &got))

		// Validate that groups was not settable by a client while
		// custom attributes like someAttr are settable by SCIM client.
		want := map[string]any{
			"active":   true,
			"userName": "scim-user-002@example.com",
			"someAttr": "someValue",
		}
		require.Equal(t, want, got)
	})

	t.Run("scim update user flow", func(t *testing.T) {
		originalUser, err := scimClient.CreateUser(t.Context(), newSCIMUser("scim-user-003"))
		require.NoError(t, err)

		const groupName5 = "group-005"
		common.CreateAccessList(t, sut,
			common.WithName(groupName5),
			common.WithAccessListType(accesslist.SCIM),
			common.WithOwners("alice-admin"),
			common.WithGrants(accesslist.Grants{Roles: []string{"access"}}),
		)
		mustPatchGroup(t, httpClient, baseURL.String(), groupName5, []map[string]any{
			{
				"op":   "add",
				"path": "members",
				"value": []map[string]any{
					{"value": originalUser.UserName},
				},
			},
		})

		u, err := scimClient.GetUser(t.Context(), originalUser.UserName)
		require.NoError(t, err)
		require.Equal(t, []string{groupName5}, userGroupNames(u))

		_, err = scimClient.UpdateUser(t.Context(), u)
		require.NoError(t, err)

		mustPatchGroup(t, httpClient, baseURL.String(), groupName5, []map[string]any{
			{
				"op":   "remove",
				"path": "members",
				"value": []map[string]any{
					{"value": originalUser.UserName},
				},
			},
		})

		u, err = scimClient.GetUser(t.Context(), originalUser.UserName)
		require.NoError(t, err)
		require.Equal(t, []string{}, userGroupNames(u))
	})
}

func userGroupNames(u *scimsdk.User) []string {
	groupsAttr, ok := u.Attributes["groups"].([]any)
	if !ok {
		return []string{}
	}
	names := make([]string, 0, len(groupsAttr))
	for _, g := range groupsAttr {
		gMap, ok := g.(map[string]any)
		if !ok {
			panic("invalid group type")
		}
		names = append(names, gMap["value"].(string))
	}
	return names
}

func newSCIMUser(username string) *scimsdk.User {
	return &scimsdk.User{
		ExternalID: username,
		UserName:   username + "@example.com",
		Active:     true,
	}
}
