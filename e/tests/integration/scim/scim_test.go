package scim

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
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
	aclClient := authClient.AccessLists

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
}

func newSCIMUser(username string) *scimsdk.User {
	return &scimsdk.User{
		ExternalID: username,
		UserName:   username + "@example.com",
		Active:     true,
	}
}
