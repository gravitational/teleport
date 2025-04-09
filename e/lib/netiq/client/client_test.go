package client

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	netiqclientmock "github.com/gravitational/teleport/e/lib/netiq/client/mock"
)

func TestGetUsers(t *testing.T) {
	t.Parallel()

	s := netiqclientmock.New()
	t.Cleanup(s.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	netIQClient, err := New(ctx, Config{
		OSPURL:                s.BaseOSPURL,
		APIURL:                s.BaseAPIURL,
		OAuthClientID:         s.OAuthClientID,
		OAuthClientSecret:     s.OAuthClientSecret,
		IdentityVaultUser:     s.IdentityVaultUser,
		IdentityVaultPassword: s.IdentityVaultPassword,
		Clock:                 clockwork.NewFakeClock(),
		InsecureSkipVerify:    true,
	})
	require.NoError(t, err, "failed to create NetIQ client"+trace.DebugReport(err))
	users, err := netIQClient.ListUsers(ctx)
	require.NoError(t, err, "failed to list users")
	require.Len(t, users, 3, "unexpected number of users")

	expectedUsers := []User{
		{
			DN:         "cn=user1,ou=users,dc=example,dc=com",
			Email:      "one@company",
			FullName:   "User One",
			IsDisabled: false,
		},
		{
			DN:         "cn=user2,ou=users,dc=example,dc=com",
			Email:      "two@company",
			FullName:   "User two",
			IsDisabled: true,
		},
		{
			DN:         "cn=user3,ou=users,dc=example,dc=com",
			Email:      "tree@company",
			FullName:   "User tree",
			IsDisabled: false,
		},
	}
	require.Equal(t, expectedUsers, users, "unexpected users")
}

func TestGetGroups(t *testing.T) {
	t.Parallel()

	s := netiqclientmock.New()
	t.Cleanup(s.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	netIQClient, err := New(ctx, Config{
		OSPURL:                s.BaseOSPURL,
		APIURL:                s.BaseAPIURL,
		OAuthClientID:         s.OAuthClientID,
		OAuthClientSecret:     s.OAuthClientSecret,
		IdentityVaultUser:     s.IdentityVaultUser,
		IdentityVaultPassword: s.IdentityVaultPassword,
		Clock:                 clockwork.NewFakeClock(),
		InsecureSkipVerify:    true,
	})
	require.NoError(t, err, "failed to create NetIQ client"+trace.DebugReport(err))
	groups, err := netIQClient.ListGroups(ctx)
	require.NoError(t, err, "failed to list groups")
	require.Len(t, groups, 3, "unexpected number of groups")

	expectedGroups := []Group{
		{
			ID:          "cn=Group1,cn=Groups,cn=Access,cn=IDVault",
			Name:        "Group1",
			Description: "Group 1",
		},
		{
			ID:          "cn=Group2,cn=Groups,cn=Access,cn=IDVault",
			Name:        "Group2",
			Description: "Group 2",
		},
		{
			ID:          "cn=Group3,cn=Groups,cn=Access,cn=IDVault",
			Name:        "Group3",
			Description: "Group 3",
		},
	}
	require.Equal(t, expectedGroups, groups, "unexpected groups")

	expectedGroupMembers := map[string][]GroupMember{
		"cn=Group1,cn=Groups,cn=Access,cn=IDVault": {
			{
				Dn:                "cn=user1,ou=users,dc=example,dc=com",
				Name:              "User One",
				IsGroupAssignment: false,
			},
			{
				Dn:                "cn=user2,ou=users,dc=example,dc=com",
				Name:              "User Two",
				IsGroupAssignment: false,
			},
		},
		"cn=Group2,cn=Groups,cn=Access,cn=IDVault": {
			{
				Dn:                "cn=user3,ou=users,dc=example,dc=com",
				Name:              "User Three",
				IsGroupAssignment: false,
			},
		},
		"cn=Group3,cn=Groups,cn=Access,cn=IDVault": nil,
	}

	for _, group := range groups {
		members, err := netIQClient.ListGroupMembers(ctx, group.ID)
		require.NoError(t, err, "failed to list group members")
		require.EqualValues(t, expectedGroupMembers[group.ID], members, "unexpected group members")
	}
}

func TestGetRoles(t *testing.T) {
	t.Parallel()

	s := netiqclientmock.New()
	t.Cleanup(s.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	netIQClient, err := New(ctx, Config{
		OSPURL:                s.BaseOSPURL,
		APIURL:                s.BaseAPIURL,
		OAuthClientID:         s.OAuthClientID,
		OAuthClientSecret:     s.OAuthClientSecret,
		IdentityVaultUser:     s.IdentityVaultUser,
		IdentityVaultPassword: s.IdentityVaultPassword,
		Clock:                 clockwork.NewFakeClock(),
		InsecureSkipVerify:    true,
	})
	require.NoError(t, err, "failed to create NetIQ client"+trace.DebugReport(err))
	roles, err := netIQClient.ListRoles(ctx)
	require.NoError(t, err, "failed to list roles")
	require.Len(t, roles, 3, "unexpected number of roles")

	expectedRoles := []Role{
		{
			ID:          "cn=Role1,cn=Roles,cn=Access,cn=IDVault",
			Name:        "Role1",
			Description: "Role 1",
		},
		{
			ID:          "cn=Role2,cn=Roles,cn=Access,cn=IDVault",
			Name:        "Role2",
			Description: "Role 2",
		},
		{
			ID:          "cn=Role3,cn=Roles,cn=Access,cn=IDVault",
			Name:        "Role3",
			Description: "Role 3",
		},
	}
	require.Equal(t, expectedRoles, roles, "unexpected roles")

	expectedRoleMembers := map[string][]RoleAssignmentStatus{
		"cn=Role1,cn=Roles,cn=Access,cn=IDVault": {
			{
				Dn:                        "cn=user1,ou=users,dc=example,dc=com",
				RecipientType:             "USER",
				RecipientTypeSubContainer: "users",
				RecipientDn:               "cn=user1,ou=users,dc=example,dc=com",
				RecipientFullName:         "User One",
				StatusCode:                "ACTIVE",
				StatusDisplay:             "Active",
				EffectiveDate:             "2021-01-01T00:00:00Z",
				ExpiryDate:                "2022-01-01T00:00:00Z",
				Description:               "Role assignment for User One",
				Grant:                     true,
			},
			{
				Dn:                        "cn=user2,ou=users,dc=example,dc=com",
				RecipientType:             "USER",
				RecipientTypeSubContainer: "users",
				RecipientDn:               "cn=user2,ou=users,dc=example,dc=com",
				RecipientFullName:         "User Two",
				StatusCode:                "ACTIVE",
				StatusDisplay:             "Active",
				EffectiveDate:             "2021-01-01T00:00:00Z",
				ExpiryDate:                "2022-01-01T00:00:00Z",
				Description:               "Role assignment for User Two",
				Grant:                     true,
			},
		},
		"cn=Role2,cn=Roles,cn=Access,cn=IDVault": {
			{
				Dn:                        "cn=user3,ou=users,dc=example,dc=com",
				RecipientType:             "USER",
				RecipientTypeSubContainer: "users",
				RecipientDn:               "cn=user3,ou=users,dc=example,dc=com",
				RecipientFullName:         "User 3",
				StatusCode:                "ACTIVE",
				StatusDisplay:             "Active",
				EffectiveDate:             "2021-01-01T00:00:00Z",
				ExpiryDate:                "2022-01-01T00:00:00Z",
				Description:               "Role assignment for User Three",
				Grant:                     true,
			},
		},
		"cn=Role3,cn=Roles,cn=Access,cn=IDVault": nil,
	}

	expectedRoleParents := map[string][]RoleRef{
		"cn=Role1,cn=Roles,cn=Access,cn=IDVault": {
			{
				ID:          "cn=Role2,cn=Roles,cn=Access,cn=IDVault",
				Name:        "Role2",
				Description: "Role 2",
			},
		},
		"cn=Role2,cn=Roles,cn=Access,cn=IDVault": nil,
		"cn=Role3,cn=Roles,cn=Access,cn=IDVault": nil,
	}

	expectedMappedResources := map[string][]ResourceRef{
		"cn=Role1,cn=Roles,cn=Access,cn=IDVault": {
			{
				ID:                 "cn=Resource1,cn=Resources,cn=Access,cn=IDVault",
				Name:               "Resource1",
				Description:        "Resource 1",
				MappingDescription: "Resource 1 mapping",
				Status:             1,
				EntityKey:          "resource1",
			},
			{
				ID:                 "cn=Resource2,cn=Resources,cn=Access,cn=IDVault",
				Name:               "Resource2",
				Description:        "Resource 2",
				MappingDescription: "Resource 2 mapping",
				Status:             1,
				EntityKey:          "resource2",
				Entitlements: []EntitlementValue{
					{
						Name:  "Entitlement1",
						ID:    "Entitlement 1",
						Value: "Value1",
					},
				},
			},
		},
		"cn=Role2,cn=Roles,cn=Access,cn=IDVault": {
			{
				ID:                 "cn=Resource3,cn=Resources,cn=Access,cn=IDVault",
				Name:               "Resource3",
				Description:        "Resource 3",
				MappingDescription: "Resource 3 mapping",
				Status:             1,
				EntityKey:          "resource3",
			},
		},
		"cn=Role3,cn=Roles,cn=Access,cn=IDVault": nil,
	}

	for _, role := range roles {
		members, err := netIQClient.ListRoleMembers(ctx, role.ID)
		require.NoError(t, err, "failed to list role members")
		require.EqualValues(t, expectedRoleMembers[role.ID], members, "unexpected role members")

		parents, err := netIQClient.ListRoleParentRoles(ctx, role.ID)
		require.NoError(t, err, "failed to list role parents")
		require.EqualValues(t, expectedRoleParents[role.ID], parents, "unexpected role parents")

		resources, err := netIQClient.ListMappedResources(ctx, role.ID)
		require.NoError(t, err, "failed to list role mapped resources")
		require.EqualValues(t, expectedMappedResources[role.ID], resources, "unexpected role mapped resources")
	}

}

func TestGetResources(t *testing.T) {
	t.Parallel()

	s := netiqclientmock.New()
	t.Cleanup(s.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	netIQClient, err := New(ctx, Config{
		OSPURL:                s.BaseOSPURL,
		APIURL:                s.BaseAPIURL,
		OAuthClientID:         s.OAuthClientID,
		OAuthClientSecret:     s.OAuthClientSecret,
		IdentityVaultUser:     s.IdentityVaultUser,
		IdentityVaultPassword: s.IdentityVaultPassword,
		Clock:                 clockwork.NewFakeClock(),
		InsecureSkipVerify:    true,
	})
	require.NoError(t, err, "failed to create NetIQ client")
	resources, err := netIQClient.ListResources(ctx)
	require.NoError(t, err, "failed to list resources")
	require.Len(t, resources, 3, "unexpected number of resources")

	expectedResources := []Resource{
		{
			ID:          "cn=Resource1,cn=Resources,cn=Access,cn=IDVault",
			Name:        "Resource1",
			Description: "Resource 1",
		},
		{
			ID:          "cn=Resource2,cn=Resources,cn=Access,cn=IDVault",
			Name:        "Resource2",
			Description: "Resource 2",
		},
		{
			ID:          "cn=Resource3,cn=Resources,cn=Access,cn=IDVault",
			Name:        "Resource3",
			Description: "Resource 3",
		},
	}

	require.Equal(t, expectedResources, resources, "unexpected resources")
}

func TestRevocation(t *testing.T) {
	s := netiqclientmock.New()
	t.Cleanup(s.Close)
	ctx := context.Background()

	clock := clockwork.NewFakeClock()

	netIQClient, err := New(ctx, Config{
		OSPURL:                s.BaseOSPURL,
		APIURL:                s.BaseAPIURL,
		OAuthClientID:         s.OAuthClientID,
		OAuthClientSecret:     s.OAuthClientSecret,
		IdentityVaultUser:     s.IdentityVaultUser,
		IdentityVaultPassword: s.IdentityVaultPassword,
		Clock:                 clock,
		InsecureSkipVerify:    true,
	})
	require.NoError(t, err, "failed to create NetIQ client"+trace.DebugReport(err))

	var tokens []string

	for i := 1; i < 15; i++ {
		token, err := netIQClient.getTokenResponse(ctx)
		require.NoError(t, err, "failed to get token response")
		require.NotEmpty(t, token.AccessToken, "access token should not be empty")
		require.NotEmpty(t, token.RefreshToken, "refresh token should not be empty")
		require.EventuallyWithT(t,
			func(t *assert.CollectT) {
				assert.Equal(t, netiqclientmock.RefreshToken(token.AccessToken, i), token.RefreshToken, "unexpected access token")
				assert.EqualValues(t, tokens, s.GetRevokedTokens(), "unexpected revoked tokens")
				assert.Equal(t, 1, s.ActiveTokens(), "unexpected number of active tokens")
			},
			5*time.Second,
			100*time.Millisecond)
		tokens = append(tokens, token.RefreshToken)

		// Simulate token expiration
		clock.Advance(time.Hour)
	}

	// Revoke the active token by closing the server
	netIQClient.Close()
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			assert.EqualValues(t, tokens, s.GetRevokedTokens(), "unexpected revoked tokens")
			assert.Equal(t, 0, s.ActiveTokens(), "unexpected number of active tokens")
		},
		5*time.Second,
		100*time.Millisecond)

}
