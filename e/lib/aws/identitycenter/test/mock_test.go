package test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
)

func TestUnifiedClientMock(t *testing.T) {
	t.Run("Ping", func(t *testing.T) {
		client := NewUnifiedMockClient(sdk.NewMockedAWSState())
		require.NoError(t, client.ViaSCIM().Ping(t.Context()))
	})

	t.Run("ListUsers", func(t *testing.T) {
		client := NewUnifiedMockClient(sdk.MockedAWSStateType{
			Users: []*sdk.User{
				{ID: "alice", UserName: "alice@example.com"},
				{ID: "bob", UserName: "bob@example.com"},
				{ID: "carol", UserName: "carol@example.com"},
				{ID: "dave", UserName: "dave@example.com"},
				{ID: "erica", UserName: "erica@example.com"},
			},
		})

		t.Run("default", func(t *testing.T) {
			resp, err := client.ViaSCIM().ListUsers(t.Context())
			require.NoError(t, err)

			var actualUsers []string
			for _, u := range resp.Users {
				actualUsers = append(actualUsers, u.ID)
			}
			require.ElementsMatch(t, actualUsers, []string{"alice", "bob", "carol", "dave", "erica"})
		})

		t.Run("with start index", func(t *testing.T) {
			resp, err := client.ViaSCIM().ListUsers(t.Context(), scimsdk.WithStartIndex(3))
			require.NoError(t, err)

			var actualUsers []string
			for _, u := range resp.Users {
				actualUsers = append(actualUsers, u.ID)
			}
			require.ElementsMatch(t, actualUsers, []string{"carol", "dave", "erica"})
		})

		t.Run("with count", func(t *testing.T) {
			resp, err := client.ViaSCIM().ListUsers(t.Context(), scimsdk.WithCount(3))
			require.NoError(t, err)

			var actualUsers []string
			for _, u := range resp.Users {
				actualUsers = append(actualUsers, u.ID)
			}
			require.ElementsMatch(t, actualUsers, []string{"alice", "bob", "carol"})
		})

		t.Run("start index out of bounds", func(t *testing.T) {
			resp, err := client.ViaSCIM().ListUsers(t.Context(), scimsdk.WithStartIndex(100))
			require.NoError(t, err)
			require.Equal(t, int32(5), resp.TotalResults)
			require.Empty(t, resp.Users)
		})

		t.Run("count out of bounds", func(t *testing.T) {
			resp, err := client.ViaSCIM().ListUsers(t.Context(), scimsdk.WithStartIndex(4), scimsdk.WithCount(100))
			require.NoError(t, err)
			require.Equal(t, int32(4), resp.StartIndex)
			require.Equal(t, int32(5), resp.TotalResults)

			var actualUsers []string
			for _, g := range resp.Users {
				actualUsers = append(actualUsers, g.ID)
			}
			require.ElementsMatch(t, actualUsers, []string{"dave", "erica"})
		})
	})

	t.Run("CreateUser", func(t *testing.T) {
		client := NewUnifiedMockClient(sdk.MockedAWSStateType{})
		scimClient := client.ViaSCIM()

		var createdUsers []*scimsdk.User
		for _, u := range makeTestUsers(100) {
			created, err := scimClient.CreateUser(t.Context(), u)
			require.NoError(t, err)
			createdUsers = append(createdUsers, created)
		}

		for _, actual := range createdUsers {
			require.True(t, slices.ContainsFunc(client.Users, byUserID(actual.ID)),
				"Mock data must contain user with ID %s", actual.ID)
			require.True(t, slices.ContainsFunc(client.Users, byUserName(actual.UserName)),
				"Mock data must contain user with name %s", actual.UserName)
		}
	})

	t.Run("GetUser", func(t *testing.T) {
		client := NewUnifiedMockClient(sdk.NewMockedAWSState())

		u, err := client.ViaSCIM().GetUser(t.Context(), "user1")
		require.NoError(t, err)
		require.Equal(t, "user_one", u.UserName)

		_, err = client.ViaSCIM().GetUser(t.Context(), "no-such-user-id")
		require.True(t, trace.IsNotFound(err),
			"Expected client to return not found error for missing user")
	})

	t.Run("GetUserByName", func(t *testing.T) {
		client := NewUnifiedMockClient(sdk.NewMockedAWSState())

		u, err := client.ViaSCIM().GetUserByUserName(t.Context(), "user_one")
		require.NoError(t, err)
		require.Equal(t, "user1", u.ID)

		_, err = client.ViaSCIM().GetUserByUserName(t.Context(), "no-such-user")
		require.True(t, trace.IsNotFound(err),
			"Expected client to return not found error for missing user")
	})

	t.Run("UpdateUser", func(t *testing.T) {
		client := NewUnifiedMockClient(sdk.NewMockedAWSState())

		u, err := client.ViaSCIM().GetUser(t.Context(), "user1")
		require.NoError(t, err)
		require.Equal(t, "user1", u.ID, "original user ID")
		require.Equal(t, "user_one", u.UserName, "original username")

		u.UserName = "darren"
		u, err = client.ViaSCIM().UpdateUser(t.Context(), u)
		require.NoError(t, err)
		require.Equal(t, "user1", u.ID, "updated user ID")
		require.Equal(t, "darren", u.UserName, "updated username")
	})

	t.Run("DeleteUser", func(t *testing.T) {
		client := NewUnifiedMockClient(sdk.MockedAWSStateType{
			Users: []*sdk.User{
				{ID: "alice", UserName: "alice@example.com"},
				{ID: "bob", UserName: "bob@example.com"},
				{ID: "carol", UserName: "carol@example.com"},
			},
			Groups: []*sdk.Group{
				{DisplayName: "Group1", ID: "group1", IdentityStoreID: "store1"},
				{DisplayName: "Group2", ID: "group2", IdentityStoreID: "store1"},
			},
			GroupMemberships: map[string][]*sdk.GroupMember{
				"group1": {
					{MemberID: "alice"},
					{MemberID: "bob"},
				},
				"group2": {
					{MemberID: "carol"},
					{MemberID: "bob"},
				},
			},
		})

		err := client.ViaSCIM().DeleteUser(t.Context(), "bob")
		require.NoError(t, err)

		requireUsersDoNotExist(t, client, "bob")
		requireNoGroupMembershipsForUser(t, client, "bob")

		requireUsersExist(t, client, "alice", "carol")
		requireUserIsMemberOfGroups(t, client, "alice", "group1")
		requireUserIsMemberOfGroups(t, client, "carol", "group2")
	})

	t.Run("CreateGroup", func(t *testing.T) {
		t.Skip("CreateGroup not yet implemented")
	})

	t.Run("GetGroup", func(t *testing.T) {
		client := NewUnifiedMockClient(sdk.MockedAWSStateType{
			Users: []*sdk.User{
				{ID: "alice", UserName: "alice@example.com"},
				{ID: "bob", UserName: "bob@example.com"},
				{ID: "carol", UserName: "carol@example.com"},
			},
			Groups: []*sdk.Group{
				{DisplayName: "Group1", ID: "group1", IdentityStoreID: "store1"},
				{DisplayName: "Group2", ID: "group2", IdentityStoreID: "store1"},
			},
			GroupMemberships: map[string][]*sdk.GroupMember{
				"group1": {
					{MemberID: "alice"},
					{MemberID: "bob"},
				},
				"group2": {
					{MemberID: "carol"},
					{MemberID: "bob"},
				},
			},
		})

		g, err := client.ViaSCIM().GetGroup(t.Context(), "group1")
		require.NoError(t, err)

		require.Equal(t, "Group1", g.DisplayName)

		expectedMembers := []*scimsdk.GroupMember{
			{ExternalID: "alice", Display: "alice@example.com", Type: scimsdk.ResourceTypeUser},
			{ExternalID: "bob", Display: "bob@example.com", Type: scimsdk.ResourceTypeUser},
		}
		require.ElementsMatch(t, expectedMembers, g.Members)
	})

	t.Run("UpdateGroup", func(t *testing.T) {
		client := NewUnifiedMockClient(sdk.MockedAWSStateType{
			Groups: []*sdk.Group{
				{DisplayName: "Group1", ID: "group1", IdentityStoreID: "store1"},
			},
		})

		updated, err := client.ViaSCIM().UpdateGroup(t.Context(), &scimsdk.Group{
			ID:          "group1",
			DisplayName: "Updated Display Name",
		})
		require.NoError(t, err)

		require.NotNil(t, updated)
		require.Equal(t, "Updated Display Name", updated.DisplayName)

		remoteGroup := (*scimClientMock)(client).getGroupByID("group1")
		require.NotNil(t, remoteGroup)
		require.Equal(t, "Updated Display Name", remoteGroup.DisplayName)
	})

	t.Run("ReplaceGroupMember", func(t *testing.T) {
		client := NewUnifiedMockClient(sdk.MockedAWSStateType{
			Users: []*sdk.User{
				{ID: "alice", UserName: "alice@example.com"},
				{ID: "bob", UserName: "bob@example.com"},
				{ID: "carol", UserName: "carol@example.com"},
				{ID: "dave", UserName: "dave@example.com"},
			},
			Groups: []*sdk.Group{
				{DisplayName: "Group1", ID: "group1", IdentityStoreID: "store1"},
			},
			GroupMemberships: map[string][]*sdk.GroupMember{
				"group1": {
					{MemberID: "alice"},
					{MemberID: "bob"},
				},
			},
		})

		err := client.ViaSCIM().ReplaceGroupMembers(t.Context(), "group1", []*scimsdk.GroupMember{
			{ExternalID: "carol"},
			{ExternalID: "dave"},
		})
		require.NoError(t, err)

		requireNoGroupMembershipsForUser(t, client, "alice")
		requireNoGroupMembershipsForUser(t, client, "bob")
		requireUserIsMemberOfGroups(t, client, "carol", "group1")
		requireUserIsMemberOfGroups(t, client, "dave", "group1")
	})

	t.Run("DeleteGroup", func(t *testing.T) {
		client := NewUnifiedMockClient(sdk.MockedAWSStateType{
			Users: []*sdk.User{
				{ID: "alice", UserName: "alice@example.com"},
				{ID: "bob", UserName: "bob@example.com"},
				{ID: "carol", UserName: "carol@example.com"},
			},
			Groups: []*sdk.Group{
				{DisplayName: "Group1", ID: "group1", IdentityStoreID: "store1"},
				{DisplayName: "Group2", ID: "group2", IdentityStoreID: "store1"},
			},
			GroupMemberships: map[string][]*sdk.GroupMember{
				"group1": {
					{MemberID: "alice"},
					{MemberID: "bob"},
				},
				"group2": {
					{MemberID: "carol"},
					{MemberID: "bob"},
				},
			},
		})

		err := client.ViaSCIM().DeleteGroup(t.Context(), "group1")
		require.NoError(t, err)

		requireGroupsDoNotExist(t, client, "group1")
		requireGroupsExist(t, client, "group2")
	})

	t.Run("List Groups", func(t *testing.T) {
		client := NewUnifiedMockClient(sdk.MockedAWSStateType{
			Users: []*sdk.User{
				{ID: "alice", UserName: "alice@example.com"},
				{ID: "bob", UserName: "bob@example.com"},
				{ID: "carol", UserName: "carol@example.com"},
			},
			Groups: []*sdk.Group{
				{DisplayName: "Group 1", ID: "group1", IdentityStoreID: "store1"},
				{DisplayName: "Group 2", ID: "group2", IdentityStoreID: "store1"},
				{DisplayName: "Group 3", ID: "group3", IdentityStoreID: "store1"},
				{DisplayName: "Group 4", ID: "group4", IdentityStoreID: "store1"},
			},
			GroupMemberships: map[string][]*sdk.GroupMember{
				"group1": {
					{MemberID: "alice"},
					{MemberID: "bob"},
				},
				"group2": {
					{MemberID: "carol"},
					{MemberID: "bob"},
				},
			},
		})

		t.Run("default", func(t *testing.T) {
			resp, err := client.ViaSCIM().ListGroups(t.Context())
			require.NoError(t, err)

			var actualGroups []string
			for _, g := range resp.Groups {
				actualGroups = append(actualGroups, g.ID)
			}
			require.ElementsMatch(t, actualGroups, []string{"group1", "group2", "group3", "group4"})
		})

		t.Run("with start index", func(t *testing.T) {
			resp, err := client.ViaSCIM().ListGroups(t.Context(), scimsdk.WithCount(2))
			require.NoError(t, err)

			var actualGroups []string
			for _, g := range resp.Groups {
				actualGroups = append(actualGroups, g.ID)
			}
			require.ElementsMatch(t, actualGroups, []string{"group1", "group2"})
		})

		t.Run("with count", func(t *testing.T) {
			resp, err := client.ViaSCIM().ListGroups(t.Context(), scimsdk.WithStartIndex(3))
			require.NoError(t, err)

			var actualGroups []string
			for _, g := range resp.Groups {
				actualGroups = append(actualGroups, g.ID)
			}
			require.ElementsMatch(t, actualGroups, []string{"group3", "group4"})
		})

		t.Run("start index out of bounds", func(t *testing.T) {
			resp, err := client.ViaSCIM().ListGroups(t.Context(), scimsdk.WithStartIndex(100))
			require.NoError(t, err)
			require.Empty(t, resp.Groups)
		})

		t.Run("count index out of bounds", func(t *testing.T) {
			resp, err := client.ViaSCIM().ListGroups(t.Context(), scimsdk.WithStartIndex(4), scimsdk.WithCount(100))
			require.NoError(t, err)

			var actualGroups []string
			for _, g := range resp.Groups {
				actualGroups = append(actualGroups, g.ID)
			}
			require.ElementsMatch(t, actualGroups, []string{"group4"})
		})
	})
}

func requireGroupsExist(t *testing.T, client *UnifiedClientMock, groupIDs ...string) {
	t.Helper()
	for _, gid := range groupIDs {
		require.True(t, slices.ContainsFunc(client.Groups, byGroupID(gid)),
			"group %q must exist", gid)
	}
}

func requireGroupsDoNotExist(t *testing.T, client *UnifiedClientMock, groupIDs ...string) {
	t.Helper()
	for _, gid := range groupIDs {
		require.False(t, slices.ContainsFunc(client.Groups, byGroupID(gid)),
			"group %q must not exist", gid)
		require.NotContains(t, client.GroupMemberships, gid)
	}
}

func requireUserIsMemberOfGroups(t *testing.T, client *UnifiedClientMock, userID string, groupIDs ...string) {
	t.Helper()
	for _, gid := range groupIDs {
		members, ok := client.GroupMemberships[gid]
		require.True(t, ok, "No group with ID %q", gid)
		require.True(t, slices.ContainsFunc(members, memberIsUser(userID)),
			"User %q must be a member og group %q", userID, gid)
	}
}

func requireNoGroupMembershipsForUser(t *testing.T, client *UnifiedClientMock, userID string) {
	t.Helper()
	for gid, members := range client.GroupMemberships {
		require.False(t, slices.ContainsFunc(members, memberIsUser(userID)),
			"User %q is member of group %q", userID, gid)
	}
}

func requireUsersDoNotExist(t *testing.T, client *UnifiedClientMock, userIDs ...string) {
	t.Helper()
	for _, userID := range userIDs {
		require.False(t, slices.ContainsFunc(client.Users, byUserID(userID)),
			"user %q must not exist", userID)
	}
}

func requireUsersExist(t *testing.T, client *UnifiedClientMock, userIDs ...string) {
	t.Helper()
	for _, userID := range userIDs {
		require.True(t, slices.ContainsFunc(client.Users, byUserID(userID)),
			"user %q must exist", userID)
	}
}

func makeTestUsers(n int) []*scimsdk.User {
	result := make([]*scimsdk.User, n)
	for i := range n {
		result[i] = &scimsdk.User{
			UserName:    fmt.Sprintf("test-username-%03d", i),
			Name:        &scimsdk.Name{FamilyName: "-", GivenName: "-"},
			DisplayName: fmt.Sprintf("test-display-name-%03d", i),
			Active:      true,
		}
	}
	return result
}
