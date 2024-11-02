package entraid

import (
	"context"
	"sort"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/msgraph"
	"github.com/gravitational/teleport/lib/services/local"
)

type fakeGraphClient struct {
	users        []*msgraph.User
	groups       []*msgraph.Group
	groupMembers map[string][]msgraph.GroupMember
}

func newFakeGraphClient() *fakeGraphClient {
	return &fakeGraphClient{
		groupMembers: make(map[string][]msgraph.GroupMember),
	}
}

func (c *fakeGraphClient) IterateGroupMembers(ctx context.Context, groupID string, f func(msgraph.GroupMember) bool) error {
	for _, m := range c.groupMembers[groupID] {
		if !f(m) {
			return nil
		}
	}
	return nil
}

func (c *fakeGraphClient) IterateGroups(ctx context.Context, f func(*msgraph.Group) bool) error {
	for _, g := range c.groups {
		if !f(g) {
			return nil
		}
	}
	return nil
}

func (c *fakeGraphClient) IterateUsers(ctx context.Context, f func(*msgraph.User) bool) error {
	for _, u := range c.users {
		if !f(u) {
			return nil
		}
	}
	return nil
}

func (c *fakeGraphClient) IterateApplications(ctx context.Context, f func(*msgraph.Application) bool) error {
	panic("not implemented")
}

func TestEntraIDService(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
	})

	sortTraits := func(u types.User) {
		for _, v := range u.GetTraits() {
			sort.Strings(v)
		}
	}

	ctx := context.Background()
	clock := clockwork.NewRealClock()
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)
	identitySvc, err := local.NewIdentityServiceV2(backend)
	require.NoError(t, err)
	alSvc, err := local.NewAccessListService(backend, clock)
	require.NoError(t, err)

	tenantID := uuid.NewString()

	samlService, err := local.NewIdentityServiceV2(backend)
	require.NoError(t, err)

	aliceID := uuid.NewString()
	bobID := uuid.NewString()
	eveID := uuid.NewString()
	daveID := uuid.NewString()
	michaelID := uuid.NewString()
	carolID := uuid.NewString()
	teamAID := uuid.NewString()
	subgroupID := uuid.NewString()

	defaultOwners := []accesslist.Owner{
		{Name: "admin", MembershipKind: accesslist.MembershipKindUser},
		{Name: "reviewer", MembershipKind: accesslist.MembershipKindUser},
	}

	const ssoConnectorID = "my-sso-connector"
	connector, err := types.NewSAMLConnector(
		ssoConnectorID,
		types.SAMLConnectorSpecV2{
			AssertionConsumerService: "http://localhost:65535/acs", // not called
			Issuer:                   "test",
			SSO:                      "https://localhost:65535/sso", // not called
			AttributesToRoles: []types.AttributeMapping{
				{Name: "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups", Value: teamAID, Roles: []string{"access"}},
				{Name: "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups", Value: subgroupID, Roles: []string{"editor"}},
			},
		})
	require.NoError(t, err)
	_, err = samlService.CreateSAMLConnector(ctx, connector)
	require.NoError(t, err)

	userMemberships := groupMembershipMap{
		aliceID: groupMembershipInfo{
			groupIds:   []string{teamAID},
			groupNames: []string{"Team A"},
		},
		bobID: groupMembershipInfo{
			groupIds:   []string{teamAID, subgroupID},
			groupNames: []string{"Team A", "foo"},
		},
	}

	// Set up data
	graphClient := newFakeGraphClient()

	// Alice does not exist in Teleport, but exists in Entra
	aliceUPN := "alice@example.com"
	aliceEntra := &msgraph.User{}
	aliceEntra.ID = &aliceID
	aliceEntra.UserPrincipalName = &aliceUPN
	aliceSAMAccountName := "alice-on-prem"
	aliceEntra.OnPremisesSAMAccountName = &aliceSAMAccountName
	graphClient.users = append(graphClient.users, aliceEntra)

	// Team A does not exist in Teleport, but exists in entra. Alice is a member
	teamAEntra := &msgraph.Group{}
	teamAEntra.ID = &teamAID
	teamAEntra.DisplayName = to.Ptr("Team A")
	graphClient.groups = append(graphClient.groups, teamAEntra)
	graphClient.groupMembers[teamAID] = []msgraph.GroupMember{aliceEntra}

	// Bob exists in both Entra and Teleport, should stay unchanged
	bobUPN := "bob@example.com"
	bobEntra := &msgraph.User{}
	bobEntra.ID = &bobID
	bobEntra.UserPrincipalName = &bobUPN
	graphClient.users = append(graphClient.users, bobEntra)

	bobTeleport, err := convertUser(bobEntra, tenantID, ssoConnectorID, userMemberships)
	require.NoError(t, err)
	bobTeleport.SetRoles([]string{"access", "editor"})
	sortTraits(bobTeleport)
	bobTeleport, err = identitySvc.CreateUser(ctx, bobTeleport)
	require.NoError(t, err)

	// Team A contains an unconvertable member.
	// It should be gracefully ignored.
	subgroup := &msgraph.Group{}
	subgroup.ID = &subgroupID
	subgroup.DisplayName = to.Ptr("foo")
	graphClient.groups = append(graphClient.groups, subgroup)
	graphClient.groupMembers[teamAID] = append(graphClient.groupMembers[teamAID], subgroup)
	graphClient.groupMembers[subgroupID] = append(graphClient.groupMembers[subgroupID], bobEntra)

	// Michael is a guest user in Entra, should be imported as a local user
	michaelUPN := "michael_someothercompany.io#EXT#@example.com"
	michaelEntra := &msgraph.User{}
	michaelEntra.ID = &michaelID
	michaelEntra.UserPrincipalName = &michaelUPN
	graphClient.users = append(graphClient.users, michaelEntra)

	michaelTeleport, err := convertUser(michaelEntra, tenantID, ssoConnectorID, userMemberships)
	require.NoError(t, err)
	michaelTeleport, err = identitySvc.CreateUser(ctx, michaelTeleport)
	require.NoError(t, err)
	require.Equal(t, "michael@someothercompany.io", michaelTeleport.GetName())

	// Carol exists in both, but was recently unassigned from Team C in Entra
	carolUPN := "carol@example.com"
	carolEntra := &msgraph.User{}
	carolEntra.ID = &carolID
	carolEntra.UserPrincipalName = &carolUPN
	graphClient.users = append(graphClient.users, carolEntra)

	carolTeleport, err := convertUser(carolEntra, tenantID, ssoConnectorID, userMemberships)
	require.NoError(t, err)
	carolTeleport, err = identitySvc.CreateUser(ctx, carolTeleport)
	require.NoError(t, err)

	// Team C exists in both, but members have changed in Entra (Carol was removed)
	teamCID := uuid.NewString()
	teamCEntra := &msgraph.Group{}
	teamCEntra.ID = &teamCID
	teamCEntra.DisplayName = to.Ptr("Team C")
	graphClient.groups = append(graphClient.groups, teamCEntra)

	teamCTeleport, err := convertGroup(teamCEntra, tenantID, defaultOwners)
	teamCTeleport.Spec.Grants.Roles = []string{"access"}
	require.NoError(t, err)

	carolTeamCTeleportMember, err := convertGroupMember(ctx, carolEntra, teamCTeleport, userMap(carolTeleport))
	require.NoError(t, err)
	_, _, err = alSvc.UpsertAccessListWithMembers(ctx, teamCTeleport, []*accesslist.AccessListMember{carolTeamCTeleportMember})
	require.NoError(t, err)

	// Dave exists in Teleport, but was removed from Entra
	daveUPN := "dave@example.com"
	daveEntra := &msgraph.User{}
	daveEntra.ID = &daveID
	daveEntra.UserPrincipalName = &daveUPN

	daveTeleport, err := convertUser(daveEntra, tenantID, ssoConnectorID, userMemberships)
	require.NoError(t, err)
	_, err = identitySvc.CreateUser(ctx, daveTeleport)
	require.NoError(t, err)

	// Eve has a local account in Teleport, should not get overwritten by her imported Entra account
	eveUPN := "eve@example.com"
	eveEntra := &msgraph.User{}
	eveEntra.ID = &eveID
	eveEntra.UserPrincipalName = &eveUPN
	graphClient.users = append(graphClient.users, eveEntra)

	eveTeleport, err := types.NewUser(eveUPN)
	require.NoError(t, err)
	eveTeleport, err = identitySvc.CreateUser(ctx, eveTeleport)
	require.NoError(t, err)

	// Frank has a local account in Teleport and no equivalent in Entra. Must not get modified or deleted.
	frankUPN := "frank@example.com"
	frankTeleport, err := types.NewUser(frankUPN)
	require.NoError(t, err)
	frankTeleport, err = identitySvc.CreateUser(ctx, frankTeleport)
	require.NoError(t, err)

	// Team F is a manually created access list in Teleport. Must not get modified or deleted.
	teamFTeleport, err := accesslist.NewAccessList(
		header.Metadata{
			Name: "team-f",
		},
		accesslist.Spec{
			Title: "Team F",
			Owners: []accesslist.Owner{
				{Name: "admin", MembershipKind: accesslist.MembershipKindUser},
			},
			Grants: accesslist.Grants{
				Traits: trait.Traits{
					"foo": []string{"bar"},
				},
			},
		},
	)
	require.NoError(t, err)
	_, err = alSvc.UpsertAccessList(ctx, teamFTeleport)
	require.NoError(t, err)
	// Refetch to update fields like ID: UpsertAccessList returns the originally passed-in object (implementation detail).
	teamFTeleport, err = alSvc.GetAccessList(ctx, teamFTeleport.GetName())
	require.NoError(t, err)

	r := &DirectoryReconciler{
		userSvc:        identitySvc,
		accessListSvc:  alSvc,
		graphClient:    graphClient,
		defaultOwners:  defaultOwners,
		ssoConnectorID: ssoConnectorID,
		tenantID:       tenantID,
		samlService:    samlService,
	}
	err = r.Reconcile(ctx)
	require.NoError(t, err)

	t.Run("alice created and assigned to team A", func(t *testing.T) {
		aliceTeleport, err := identitySvc.GetUser(ctx, aliceUPN, false)
		require.NoError(t, err)
		require.Equal(t, types.OriginEntraID, aliceTeleport.GetAllLabels()[types.OriginLabel])
		require.Equal(t, tenantID, aliceTeleport.GetAllLabels()[types.EntraTenantIDLabel])
		require.Equal(t, aliceUPN, aliceTeleport.GetAllLabels()[types.EntraUPNLabel])
		require.Equal(t, aliceSAMAccountName, aliceTeleport.GetAllLabels()[types.EntraSAMAccountNameLabel])
		require.NotNil(t, aliceTeleport.GetCreatedBy().Connector)
		require.Equal(t, ssoConnectorID, aliceTeleport.GetCreatedBy().Connector.ID)
		require.Equal(t, []string{"access"}, aliceTeleport.GetRoles())
		require.Equal(t, map[string][]string{
			"http://schemas.microsoft.com/ws/2008/06/identity/claims/groups": {
				teamAID,
			},
			"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name": {"alice@example.com"},
		}, aliceTeleport.GetTraits())

		teamATeleportExpected, err := convertGroup(teamAEntra, tenantID, defaultOwners)
		require.NoError(t, err)
		teamATeleport, err := alSvc.GetAccessList(ctx, teamATeleportExpected.GetName())
		require.NoError(t, err)
		require.Equal(t, "Team A", teamATeleport.Spec.Title)
		require.Equal(t, types.OriginEntraID, teamATeleport.GetAllLabels()[types.OriginLabel])
		require.Equal(t, tenantID, teamATeleport.GetAllLabels()[types.EntraTenantIDLabel])
		require.Equal(t, defaultOwners, teamATeleport.GetOwners())
		require.Equal(t, []string{teamAID}, teamATeleport.GetGrants().Traits[eteleport.EntraMemberOfGroupTrait])

		teamAMembers, pageToken, err := alSvc.ListAccessListMembers(ctx, teamATeleport.GetName(), 1, "")
		require.NoError(t, err)
		require.Empty(t, pageToken, "Team A access list should only have one member")
		require.Len(t, teamAMembers, 1)
		require.Equal(t, aliceUPN, teamAMembers[0].GetName())
	})

	t.Run("bob unchanged", func(t *testing.T) {
		bobTeleportNew, err := identitySvc.GetUser(ctx, bobUPN, false)
		require.NoError(t, err)
		require.Empty(t, compareResources(t, bobTeleport, bobTeleportNew))
	})

	t.Run("carol unassigned from Team C", func(t *testing.T) {
		carolTeleportNew, err := identitySvc.GetUser(ctx, carolUPN, false)
		require.NoError(t, err)
		require.Empty(t, compareResources(t, carolTeleport, carolTeleportNew))

		teamCMembers, pageToken, err := alSvc.ListAccessListMembers(ctx, teamCTeleport.GetName(), 1, "")
		require.NoError(t, err)
		require.Empty(t, pageToken, "Team C access list should have no members")
		require.Empty(t, teamCMembers)

		teamCTeleportNew, err := alSvc.GetAccessList(ctx, teamCTeleport.GetName())
		require.NoError(t, err)
		require.Equal(t, []string{"access"}, teamCTeleportNew.GetGrants().Roles)
	})

	t.Run("dave was removed", func(t *testing.T) {
		_, err := identitySvc.GetUser(ctx, daveUPN, false)
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err), "expected a 'not found' error, but got %s instead", err)
	})

	t.Run("eve unchanged", func(t *testing.T) {
		eveTeleportNew, err := identitySvc.GetUser(ctx, eveUPN, false)
		require.NoError(t, err)
		require.Empty(t, compareResources(t, eveTeleport, eveTeleportNew))
	})

	t.Run("frank unchanged", func(t *testing.T) {
		frankTeleportNew, err := identitySvc.GetUser(ctx, frankUPN, false)
		require.NoError(t, err)
		require.Empty(t, compareResources(t, frankTeleport, frankTeleportNew))
	})

	t.Run("Team F unchanged", func(t *testing.T) {
		teamFTeleportNew, err := alSvc.GetAccessList(ctx, teamFTeleport.GetName())
		require.NoError(t, err)
		require.Empty(t, compareResources(t, teamFTeleport, teamFTeleportNew))
	})
}

func compareResources(t *testing.T, expected, actual types.Resource) string {
	t.Helper()

	// This compares including ID/Revision.
	// We expect a full match including these properties as a way to verify that Update() was not called.
	return cmp.Diff(expected, actual)
}

// userMap returns an entra ID -> teleport User look up map with a single object
// for use in calls to convertGroupMember() in tests
func userMap(u types.User) map[entraUniqueID]types.User {
	l := entraUniqueID(u.GetAllLabels()[types.EntraUniqueIDLabel])
	return map[entraUniqueID]types.User{l: u}
}
