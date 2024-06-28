package entraid

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services/local"
)

type fakeGraphClient struct {
	users        []models.Userable
	groups       []models.Groupable
	groupMembers map[string][]models.DirectoryObjectable
}

func newFakeGraphClient() *fakeGraphClient {
	return &fakeGraphClient{
		groupMembers: make(map[string][]models.DirectoryObjectable),
	}
}

func (c *fakeGraphClient) IterateGroupMembers(ctx context.Context, groupID string, f func(models.DirectoryObjectable) bool) error {
	for _, m := range c.groupMembers[groupID] {
		if !f(m) {
			return nil
		}
	}
	return nil
}

func (c *fakeGraphClient) IterateGroups(ctx context.Context, f func(models.Groupable) bool) error {
	for _, g := range c.groups {
		if !f(g) {
			return nil
		}
	}
	return nil
}

func (c *fakeGraphClient) IterateUsers(ctx context.Context, f func(models.Userable) bool) error {
	for _, u := range c.users {
		if !f(u) {
			return nil
		}
	}
	return nil
}

func (c *fakeGraphClient) IterateApplications(ctx context.Context, f func(models.Applicationable) bool) error {
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
	ctx := context.Background()
	clock := clockwork.NewRealClock()
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)
	identitySvc := local.NewIdentityService(backend)
	alSvc, err := local.NewAccessListService(backend, clock)
	require.NoError(t, err)

	tenantID := uuid.NewString()
	defaultOwners := []accesslist.Owner{{Name: "admin"}, {Name: "reviewer"}}
	const ssoConnectorID = "my-sso-connector"

	// Set up data
	graphClient := newFakeGraphClient()

	// Alice does not exist in Teleport, but exists in Entra
	aliceID := uuid.NewString()
	aliceUPN := "alice@example.com"
	aliceEntra := models.NewUser()
	aliceEntra.SetId(&aliceID)
	aliceEntra.SetUserPrincipalName(&aliceUPN)
	aliceSAMAccountName := "alice-on-prem"
	aliceEntra.SetOnPremisesSamAccountName(&aliceSAMAccountName)
	graphClient.users = append(graphClient.users, aliceEntra)

	// Team A does not exist in Teleport, but exists in entra. Alice is a member
	teamAID := uuid.NewString()
	teamAEntra := models.NewGroup()
	teamAEntra.SetId(&teamAID)
	teamAEntra.SetDisplayName(to.Ptr("Team A"))
	graphClient.groups = append(graphClient.groups, teamAEntra)
	graphClient.groupMembers[teamAID] = []models.DirectoryObjectable{aliceEntra}

	// Team A contains an unconvertable member.
	// It should be gracefully ignored.
	deviceID := uuid.NewString()
	device := models.NewDevice()
	device.SetId(&deviceID)
	graphClient.groupMembers[teamAID] = append(graphClient.groupMembers[teamAID], device)

	// Bob exists in both Entra and Teleport, should stay unchanged
	bobID := uuid.NewString()
	bobUPN := "bob@example.com"
	bobEntra := models.NewUser()
	bobEntra.SetId(&bobID)
	bobEntra.SetUserPrincipalName(&bobUPN)
	graphClient.users = append(graphClient.users, bobEntra)

	bobTeleport, err := convertUser(bobEntra, tenantID, ssoConnectorID)
	require.NoError(t, err)
	bobTeleport, err = identitySvc.CreateUser(ctx, bobTeleport)
	require.NoError(t, err)

	// Carol exists in both, but was recently unassigned from Team C in Entra
	carolID := uuid.NewString()
	carolUPN := "carol@example.com"
	carolEntra := models.NewUser()
	carolEntra.SetId(&carolID)
	carolEntra.SetUserPrincipalName(&carolUPN)
	graphClient.users = append(graphClient.users, carolEntra)

	carolTeleport, err := convertUser(carolEntra, tenantID, ssoConnectorID)
	require.NoError(t, err)
	carolTeleport, err = identitySvc.CreateUser(ctx, carolTeleport)
	require.NoError(t, err)

	// Team C exists in both, but members have changed in Entra (Carol was removed)
	teamCID := uuid.NewString()
	teamCEntra := models.NewGroup()
	teamCEntra.SetId(&teamCID)
	teamCEntra.SetDisplayName(to.Ptr("Team C"))
	graphClient.groups = append(graphClient.groups, teamCEntra)

	teamCTeleport, err := convertGroup(teamCEntra, tenantID, defaultOwners)
	require.NoError(t, err)

	carolTeamCTeleportMember, err := convertGroupMember(ctx, carolEntra, teamCTeleport, userMap(carolTeleport))
	require.NoError(t, err)
	_, _, err = alSvc.UpsertAccessListWithMembers(ctx, teamCTeleport, []*accesslist.AccessListMember{carolTeamCTeleportMember})
	require.NoError(t, err)

	// Dave exists in Teleport, but was removed from Entra
	daveID := uuid.NewString()
	daveUPN := "dave@example.com"
	daveEntra := models.NewUser()
	daveEntra.SetId(&daveID)
	daveEntra.SetUserPrincipalName(&daveUPN)

	daveTeleport, err := convertUser(daveEntra, tenantID, ssoConnectorID)
	require.NoError(t, err)
	_, err = identitySvc.CreateUser(ctx, daveTeleport)
	require.NoError(t, err)

	// Eve has a local account in Teleport, should not get overwritten by her imported Entra account
	eveID := uuid.NewString()
	eveUPN := "eve@example.com"
	eveEntra := models.NewUser()
	eveEntra.SetId(&eveID)
	eveEntra.SetUserPrincipalName(&eveUPN)
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
				{Name: "admin"},
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
