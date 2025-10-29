package scim

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	typescommon "github.com/gravitational/teleport/api/types/common"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
)

func TestSCIMGeneric(t *testing.T) {
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

	t.Run("Add single member to group", func(t *testing.T) {
		group, err := scimClient.GetGroup(t.Context(), createdGroup.ID)
		require.NoError(t, err)

		group.Members = append(group.Members, &scimsdk.GroupMember{
			ExternalID: scimUser1.UserName,
		})
		group, err = scimClient.UpdateGroup(t.Context(), group)
		require.NoError(t, err)

		members, _, err := aclClient.ListAccessListMembers(t.Context(), group.ID, 0, "")
		require.NoError(t, err)
		require.Len(t, members, 1)
		require.Equal(t, scimUser1.UserName, members[0].GetName())
		require.Equal(t, "SCIM", members[0].Spec.AddedBy)
	})

	t.Run("Add second member to group", func(t *testing.T) {
		createdGroup.Members = []*scimsdk.GroupMember{
			{ExternalID: scimUser1.UserName},
			{ExternalID: scimUser2.UserName},
		}
		_, err := scimClient.UpdateGroup(t.Context(), createdGroup)
		require.NoError(t, err)

		members, _, err := aclClient.ListAccessListMembers(t.Context(), createdGroup.ID, 0, "")
		require.NoError(t, err)
		require.Len(t, members, 2)
	})

	t.Run("Remove first member from group", func(t *testing.T) {
		createdGroup.Members = []*scimsdk.GroupMember{
			{ExternalID: scimUser2.UserName},
		}
		_, err := scimClient.UpdateGroup(t.Context(), createdGroup)
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
}

func TestOIDCConnector(t *testing.T) {
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
