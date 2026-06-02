package directory

import (
	"context"
	"errors"
	"iter"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"sync"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/e/lib/mdmsync"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/msgraph"
	"github.com/gravitational/teleport/lib/msgraph/models"
	"github.com/gravitational/teleport/lib/msgraph/msgraphtest"
	"github.com/gravitational/teleport/lib/plugins/filter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

type fakeGraphClient struct {
	users        []*models.User
	groups       []*models.Group
	groupMembers map[string][]models.GroupMember
	groupOwners  map[string][]*models.User
	applications []*models.Application
}

func newFakeGraphClient() *fakeGraphClient {
	return &fakeGraphClient{
		groupMembers: make(map[string][]models.GroupMember),
		groupOwners:  make(map[string][]*models.User),
	}
}

func (c *fakeGraphClient) IterateGroupMembers(ctx context.Context, groupID string, f func(models.GroupMember) bool, opts ...msgraph.IterateOpt) error {
	for _, m := range c.groupMembers[groupID] {
		if !f(m) {
			return nil
		}
	}
	return nil
}

func (c *fakeGraphClient) IterateGroupOwners(ctx context.Context, groupID string, f func(*models.User) bool, opts ...msgraph.IterateOpt) error {
	for _, m := range c.groupOwners[groupID] {
		if !f(m) {
			return nil
		}
	}
	return nil
}

func (c *fakeGraphClient) IterateGroups(ctx context.Context, f func(*models.Group) bool, opts ...msgraph.IterateOpt) error {
	for _, g := range c.groups {
		if !f(g) {
			break
		}
	}
	return nil
}

func (c *fakeGraphClient) IterateUsers(ctx context.Context, f func(*models.User) bool, opts ...msgraph.IterateOpt) error {
	for _, u := range c.users {
		if !f(u) {
			return nil
		}
	}
	return nil
}

func (c *fakeGraphClient) IterateApplications(ctx context.Context, f func(*models.Application) bool, opts ...msgraph.IterateOpt) error {
	panic("not implemented")
}

func (c *fakeGraphClient) GetApplication(ctx context.Context, appID string) (*models.Application, error) {
	for _, app := range c.applications {
		if *app.AppID == appID {
			return app, nil
		}
	}

	return nil, trace.NotFound("application %q not found", appID)
}

func (c *fakeGraphClient) IterateUserDeltas(ctx context.Context, endpoint string, ds msgraph.DeltaStore) iter.Seq2[*models.ListUsersDeltaResponse, error] {
	return func(yield func(*models.ListUsersDeltaResponse, error) bool) {
		yield(nil, trace.NotImplemented("not implemented"))
	}
}

func (c *fakeGraphClient) IterateGroupDeltas(ctx context.Context, endpoint string, ds msgraph.DeltaStore) iter.Seq2[*models.ListGroupsDeltaResponse, error] {
	return func(yield func(*models.ListGroupsDeltaResponse, error) bool) {
		yield(nil, trace.NotImplemented("not implemented"))
	}
}

func (c *fakeGraphClient) SetupLatestDelta(context.Context, string, msgraph.DeltaStore, ...msgraph.IterateOpt) error {
	return nil
}

func TestDirectoryReconciler(t *testing.T) {
	t.Parallel()
	sortTraits := func(u types.User) {
		for _, v := range u.GetTraits() {
			sort.Strings(v)
		}
	}

	ctx := t.Context()
	graphClient := newFakeGraphClient()
	teamAID := uuid.NewString()
	subgroupID := uuid.NewString()
	connector := newSAMLConnector(t, "my-sso-connector", teamAID, subgroupID)
	env := NewEnv(t, graphClient, connector)

	aliceID := uuid.NewString()
	bobID := uuid.NewString()
	eveID := uuid.NewString()
	daveID := uuid.NewString()
	michaelID := uuid.NewString()
	carolID := uuid.NewString()

	userMemberships := groupMembershipMap{
		aliceID: groupMembershipInfo{
			groupIds:   []string{teamAID},
			groupNames: []string{teamAID},
		},
		bobID: groupMembershipInfo{
			groupIds:   []string{teamAID, subgroupID},
			groupNames: []string{teamAID, subgroupID},
		},
	}

	// Alice does not exist in Teleport, but exists in Entra
	aliceUPN := "alice@example.com"
	aliceEntra := &models.User{}
	aliceEntra.ID = &aliceID
	aliceEntra.UserPrincipalName = &aliceUPN
	aliceEntra.DisplayName = to.Ptr("Alice Smith")
	aliceEntra.GivenName = to.Ptr("Alice")
	aliceEntra.Surname = to.Ptr("Smith")
	aliceSAMAccountName := "alice-on-prem"
	aliceEntra.OnPremisesSAMAccountName = &aliceSAMAccountName
	graphClient.users = append(graphClient.users, aliceEntra)

	// Team A does not exist in Teleport, but exists in entra. Alice is a member
	teamAEntra := newEntraGroup(t, teamAID, "Team A")
	graphClient.groups = append(graphClient.groups, teamAEntra)
	graphClient.groupMembers[teamAID] = []models.GroupMember{aliceEntra}

	// Bob exists in both Entra and Teleport, should stay unchanged
	bobEntra := entraUser(t, bobID, "bob@example.com")
	graphClient.users = append(graphClient.users, bobEntra)

	cfg := userConfig{
		tenantID:       env.cfg.TenantID,
		ssoConnectorID: env.cfg.SSOConnectorID,
		emitAsRoles:    false,
	}
	bobTeleport, err := convertUser(bobEntra, userMemberships, cfg)
	require.NoError(t, err)
	bobTeleport.SetRoles([]string{"access", "editor"})
	sortTraits(bobTeleport)
	bobTeleport, err = env.cfg.AccessPoint.CreateUser(ctx, bobTeleport)
	require.NoError(t, err)

	// Team A contains an unconvertable member.
	// It should be gracefully ignored.
	subgroup := newEntraGroup(t, subgroupID, "foo")
	graphClient.groups = append(graphClient.groups, subgroup)
	graphClient.groupMembers[teamAID] = append(graphClient.groupMembers[teamAID], subgroup)
	graphClient.groupMembers[subgroupID] = append(graphClient.groupMembers[subgroupID], bobEntra)

	// Michael is a guest user in Entra, should be imported as a local user
	michaelEntra := entraUser(t, michaelID, "michael_someothercompany.io#EXT#@example.com")
	graphClient.users = append(graphClient.users, michaelEntra)

	michaelTeleport, err := convertUser(michaelEntra, userMemberships, cfg)
	require.NoError(t, err)
	michaelTeleport, err = env.cfg.AccessPoint.CreateUser(ctx, michaelTeleport)
	require.NoError(t, err)
	require.Equal(t, "michael@someothercompany.io", michaelTeleport.GetName())

	// Carol exists in both, but was recently unassigned from Team C in Entra
	carolEntra := entraUser(t, carolID, "carol@example.com")
	graphClient.users = append(graphClient.users, carolEntra)

	carolTeleport, err := convertUser(carolEntra, userMemberships, cfg)
	require.NoError(t, err)
	carolTeleport, err = env.cfg.AccessPoint.CreateUser(ctx, carolTeleport)
	require.NoError(t, err)

	// Team C exists in both, but members have changed in Entra (Carol was removed)
	teamCEntra := newEntraGroup(t, uuid.NewString(), "Team C")
	graphClient.groups = append(graphClient.groups, teamCEntra)

	_, teamCTeleport, err := convertGroup(teamCEntra, env.cfg.TenantID, env.cfg.DefaultOwners)
	teamCTeleport.Spec.Grants.Roles = []string{"access"}
	require.NoError(t, err)

	carolTeamCTeleportMember, err := convertGroupMember(carolEntra, teamCTeleport, userMap(carolTeleport), nil)
	require.NoError(t, err)
	_, _, err = env.aclSvc.UpsertAccessListWithMembers(ctx, teamCTeleport, []*accesslist.AccessListMember{carolTeamCTeleportMember})
	require.NoError(t, err)

	// Dave exists in Teleport, but was removed from Entra
	daveEntra := entraUser(t, daveID, "dave@example.com")

	daveTeleport, err := convertUser(daveEntra, userMemberships, cfg)
	require.NoError(t, err)
	_, err = env.cfg.AccessPoint.CreateUser(ctx, daveTeleport)
	require.NoError(t, err)

	// Eve has a local account in Teleport, should not get overwritten by her imported Entra account
	eveEntra := entraUser(t, eveID, "eve@example.com")
	graphClient.users = append(graphClient.users, eveEntra)

	eveTeleport, err := types.NewUser(*eveEntra.UserPrincipalName)
	require.NoError(t, err)
	eveTeleport, err = env.cfg.AccessPoint.CreateUser(ctx, eveTeleport)
	require.NoError(t, err)

	// Frank has a local account in Teleport and no equivalent in Entra. Must not get modified or deleted.
	frankUPN := "frank@example.com"
	frankTeleport, err := types.NewUser(frankUPN)
	require.NoError(t, err)
	frankTeleport, err = env.cfg.AccessPoint.CreateUser(ctx, frankTeleport)
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
	_, err = env.aclSvc.UpsertAccessList(ctx, teamFTeleport)
	require.NoError(t, err)
	// Refetch to update fields like ID: UpsertAccessList returns the originally passed-in object (implementation detail).
	teamFTeleport, err = env.aclSvc.GetAccessList(ctx, teamFTeleport.GetName())
	require.NoError(t, err)

	r, err := New(env.cfg)
	require.NoError(t, err)

	result, err := r.Reconcile(ctx, mdmsync.SyncModeFull)
	require.NoError(t, err)
	require.ErrorContains(t, result.ErrSkippedResources, "eve@example.com")

	t.Run("alice created and assigned to team A", func(t *testing.T) {
		aliceTeleport, err := env.identitySvc.GetUser(ctx, aliceUPN, false)
		require.NoError(t, err)
		require.Equal(t, types.OriginEntraID, aliceTeleport.GetAllLabels()[types.OriginLabel])
		require.Equal(t, env.cfg.TenantID, aliceTeleport.GetAllLabels()[types.EntraTenantIDLabel])
		require.Equal(t, aliceUPN, aliceTeleport.GetAllLabels()[types.EntraUPNLabel])
		require.Equal(t, aliceSAMAccountName, aliceTeleport.GetAllLabels()[types.EntraSAMAccountNameLabel])
		require.NotNil(t, aliceTeleport.GetCreatedBy().Connector)
		require.Equal(t, env.cfg.SSOConnectorID, aliceTeleport.GetCreatedBy().Connector.ID)
		require.Equal(t, []string{"access"}, aliceTeleport.GetRoles())
		require.Equal(t, map[string][]string{
			"http://schemas.microsoft.com/ws/2008/06/identity/claims/groups": {
				teamAID,
			},
			"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name":      {"alice@example.com"},
			"http://schemas.microsoft.com/identity/claims/tenantid":           {env.cfg.TenantID},
			"http://schemas.microsoft.com/identity/claims/objectidentifier":   {aliceID},
			"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname": {"Alice"},
			"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname":   {"Smith"},
			"http://schemas.microsoft.com/identity/claims/displayname":        {"Alice Smith"},
		}, aliceTeleport.GetTraits())

		teamATeleport := requireAccessListForEntraGroupExists(t, env.aclSvc, teamAEntra)
		require.NoError(t, err)
		require.Equal(t, "Team A", teamATeleport.Spec.Title)
		require.Equal(t, types.OriginEntraID, teamATeleport.GetAllLabels()[types.OriginLabel])
		require.Equal(t, env.cfg.TenantID, teamATeleport.GetAllLabels()[types.EntraTenantIDLabel])
		require.Equal(t, env.cfg.DefaultOwners, teamATeleport.GetOwners())
		require.Equal(t, []string{teamAID}, teamATeleport.GetGrants().Traits[eteleport.EntraMemberOfGroupTrait])

		teamAMembers, pageToken, err := env.aclSvc.ListAccessListMembers(ctx, teamATeleport.GetName(), 2, "")
		require.NoError(t, err)
		require.Empty(t, pageToken, "Team A access list should only have two member")
		require.Len(t, teamAMembers, 2)

		// assume the first entry is the user. This is generally true because AL names are
		// static.
		subGroupMember := teamAMembers[0]
		aliceMember := teamAMembers[1]
		// if it's not true, swap the values
		if aliceMember.Spec.MembershipKind == accesslistv1.MembershipKind_MEMBERSHIP_KIND_LIST.String() {
			subGroupMember, aliceMember = aliceMember, subGroupMember
		}

		require.Equal(t, aliceUPN, aliceMember.GetName())
		require.Equal(t, accesslistv1.MembershipKind_MEMBERSHIP_KIND_USER.String(), aliceMember.Spec.MembershipKind)

		require.Equal(t, accessListName(*subgroup.DisplayName, *subgroup.ID), subGroupMember.GetName())
		require.Equal(t, accesslistv1.MembershipKind_MEMBERSHIP_KIND_LIST.String(), subGroupMember.Spec.MembershipKind)
	})

	t.Run("bob unchanged", func(t *testing.T) {
		bobTeleportNew, err := env.identitySvc.GetUser(ctx, *bobEntra.UserPrincipalName, false)
		require.NoError(t, err)
		require.Empty(t, compareResources(t, bobTeleport, bobTeleportNew))
	})

	t.Run("carol unassigned from Team C", func(t *testing.T) {
		carolTeleportNew, err := env.identitySvc.GetUser(ctx, *carolEntra.UserPrincipalName, false)
		require.NoError(t, err)
		require.Empty(t, compareResources(t, carolTeleport, carolTeleportNew))

		teamCMembers, pageToken, err := env.aclSvc.ListAccessListMembers(ctx, teamCTeleport.GetName(), 1, "")
		require.NoError(t, err)
		require.Empty(t, pageToken, "Team C access list should have no members")
		require.Empty(t, teamCMembers)

		teamCTeleportNew, err := env.aclSvc.GetAccessList(ctx, teamCTeleport.GetName())
		require.NoError(t, err)
		require.Equal(t, []string{"access"}, teamCTeleportNew.GetGrants().Roles)
	})

	t.Run("dave was removed", func(t *testing.T) {
		_, err := env.identitySvc.GetUser(ctx, *daveEntra.UserPrincipalName, false)
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err), "expected a 'not found' error, but got %s instead", err)
	})

	t.Run("eve unchanged", func(t *testing.T) {
		eveTeleportNew, err := env.identitySvc.GetUser(ctx, *eveEntra.UserPrincipalName, false)
		require.NoError(t, err)
		require.Empty(t, compareResources(t, eveTeleport, eveTeleportNew))
	})

	t.Run("frank unchanged", func(t *testing.T) {
		frankTeleportNew, err := env.identitySvc.GetUser(ctx, frankUPN, false)
		require.NoError(t, err)
		require.Empty(t, compareResources(t, frankTeleport, frankTeleportNew))
	})

	t.Run("Team F unchanged", func(t *testing.T) {
		teamFTeleportNew, err := env.aclSvc.GetAccessList(ctx, teamFTeleport.GetName())
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

func Test_getGroupNameBuilderFunc(t *testing.T) {
	t.Parallel()
	const (
		groupID        = "uuid"
		samAccountName = "foo"
		netBiosName    = "bar"
		domainName     = "baz"
	)

	tests := []struct {
		name            string
		optionalClaims  *models.OptionalClaims
		group           *models.Group
		emitAsRoles     bool
		groupTraitValue string
	}{
		{
			name:           "no optional claims",
			optionalClaims: nil,
			group: &models.Group{
				DirectoryObject: models.DirectoryObject{
					ID: to.Ptr(groupID),
				},
				OnPremisesDomainName:     to.Ptr(domainName),
				OnPremisesNetBiosName:    to.Ptr(netBiosName),
				OnPremisesSamAccountName: to.Ptr(samAccountName),
			},
			groupTraitValue: groupID,
			emitAsRoles:     false,
		},
		{
			name:           "no optional claims but set",
			optionalClaims: &models.OptionalClaims{},
			group: &models.Group{
				DirectoryObject: models.DirectoryObject{
					ID: to.Ptr(groupID),
				},
				OnPremisesDomainName:     to.Ptr(domainName),
				OnPremisesNetBiosName:    to.Ptr(netBiosName),
				OnPremisesSamAccountName: to.Ptr(samAccountName),
			},
			groupTraitValue: groupID,
			emitAsRoles:     false,
		},
		{
			name: "use sam account name",
			optionalClaims: &models.OptionalClaims{
				SAML2Token: []models.OptionalClaim{
					{
						Name:                 to.Ptr("groups"),
						AdditionalProperties: []string{"sam_account_name"},
					},
				},
			},
			group: &models.Group{
				DirectoryObject: models.DirectoryObject{
					ID: to.Ptr(groupID),
				},
				OnPremisesDomainName:     to.Ptr(domainName),
				OnPremisesNetBiosName:    to.Ptr(netBiosName),
				OnPremisesSamAccountName: to.Ptr(samAccountName),
			},
			groupTraitValue: samAccountName,
			emitAsRoles:     false,
		},
		{
			name: "use sam account name but not set",
			optionalClaims: &models.OptionalClaims{
				SAML2Token: []models.OptionalClaim{
					{
						Name:                 to.Ptr("groups"),
						AdditionalProperties: []string{"sam_account_name"},
					},
				},
			},
			group: &models.Group{
				DirectoryObject: models.DirectoryObject{
					ID: to.Ptr(groupID),
				},
				OnPremisesDomainName:  to.Ptr(domainName),
				OnPremisesNetBiosName: to.Ptr(netBiosName),
			},
			groupTraitValue: groupID,
			emitAsRoles:     false,
		},
		{
			name: "use net bios sam account name",
			optionalClaims: &models.OptionalClaims{
				SAML2Token: []models.OptionalClaim{
					{
						Name:                 to.Ptr("groups"),
						AdditionalProperties: []string{"netbios_domain_and_sam_account_name"},
					},
				},
			},
			group: &models.Group{
				DirectoryObject: models.DirectoryObject{
					ID: to.Ptr(groupID),
				},
				OnPremisesDomainName:     to.Ptr(domainName),
				OnPremisesNetBiosName:    to.Ptr(netBiosName),
				OnPremisesSamAccountName: to.Ptr(samAccountName),
			},
			groupTraitValue: netBiosName + `\` + samAccountName,
			emitAsRoles:     false,
		},
		{
			name: "use net bios sam account name but not set",
			optionalClaims: &models.OptionalClaims{
				SAML2Token: []models.OptionalClaim{
					{
						Name:                 to.Ptr("groups"),
						AdditionalProperties: []string{"netbios_domain_and_sam_account_name"},
					},
				},
			},
			group: &models.Group{
				DirectoryObject: models.DirectoryObject{
					ID: to.Ptr(groupID),
				},
				OnPremisesDomainName:  to.Ptr(domainName),
				OnPremisesNetBiosName: to.Ptr(netBiosName),
			},
			groupTraitValue: groupID,
			emitAsRoles:     false,
		},
		{
			name: "use net bios sam account name but not set",
			optionalClaims: &models.OptionalClaims{
				SAML2Token: []models.OptionalClaim{
					{
						Name:                 to.Ptr("groups"),
						AdditionalProperties: []string{"netbios_domain_and_sam_account_name"},
					},
				},
			},
			group: &models.Group{
				DirectoryObject: models.DirectoryObject{
					ID: to.Ptr(groupID),
				},
				OnPremisesSamAccountName: to.Ptr(samAccountName),
				OnPremisesDomainName:     to.Ptr(domainName),
			},
			groupTraitValue: groupID,
			emitAsRoles:     false,
		},
		{
			name: "use domain name sam account name",
			optionalClaims: &models.OptionalClaims{
				SAML2Token: []models.OptionalClaim{
					{
						Name:                 to.Ptr("groups"),
						AdditionalProperties: []string{"dns_domain_and_sam_account_name"},
					},
				},
			},
			group: &models.Group{
				DirectoryObject: models.DirectoryObject{
					ID: to.Ptr(groupID),
				},
				OnPremisesDomainName:     to.Ptr(domainName),
				OnPremisesNetBiosName:    to.Ptr(netBiosName),
				OnPremisesSamAccountName: to.Ptr(samAccountName),
			},
			groupTraitValue: domainName + `\` + samAccountName,
			emitAsRoles:     false,
		},
		{
			name: "use domain name sam account name but not set",
			optionalClaims: &models.OptionalClaims{
				SAML2Token: []models.OptionalClaim{
					{
						Name:                 to.Ptr("groups"),
						AdditionalProperties: []string{"dns_domain_and_sam_account_name"},
					},
				},
			},
			group: &models.Group{
				DirectoryObject: models.DirectoryObject{
					ID: to.Ptr(groupID),
				},
				OnPremisesDomainName:  to.Ptr(domainName),
				OnPremisesNetBiosName: to.Ptr(netBiosName),
			},
			groupTraitValue: groupID,
			emitAsRoles:     false,
		},
		{
			name: "use domain name sam account name but not set",
			optionalClaims: &models.OptionalClaims{
				SAML2Token: []models.OptionalClaim{
					{
						Name:                 to.Ptr("groups"),
						AdditionalProperties: []string{"dns_domain_and_sam_account_name"},
					},
				},
			},
			group: &models.Group{
				DirectoryObject: models.DirectoryObject{
					ID: to.Ptr(groupID),
				},
				OnPremisesSamAccountName: to.Ptr(samAccountName),
				OnPremisesNetBiosName:    to.Ptr(netBiosName),
			},
			groupTraitValue: groupID,
			emitAsRoles:     false,
		},
		{
			name: "use domain name sam account name but not set",
			optionalClaims: &models.OptionalClaims{
				SAML2Token: []models.OptionalClaim{
					{
						Name:                 to.Ptr("groups"),
						AdditionalProperties: []string{"dns_domain_and_sam_account_name", "emit_as_roles"},
					},
				},
			},
			group: &models.Group{
				DirectoryObject: models.DirectoryObject{
					ID: to.Ptr(groupID),
				},
				OnPremisesSamAccountName: to.Ptr(samAccountName),
				OnPremisesNetBiosName:    to.Ptr(netBiosName),
			},
			groupTraitValue: groupID,
			emitAsRoles:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &models.Application{
				OptionalClaims: tt.optionalClaims,
			}
			emitAsRoles, f := getGroupNameBuilderFunc(app)
			require.Equal(t, tt.emitAsRoles, emitAsRoles)
			require.Equal(t, tt.groupTraitValue, f(tt.group))
		})
	}
}

func TestUserSync(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	t.Run("Create user succeeds", func(t *testing.T) {
		graphClient := newFakeGraphClient()
		env := NewEnv(t, graphClient, nil /* custom saml connector */)
		graphClient.users = []*models.User{
			entraUser(t, "u1", "alice@example.com"),
			entraUser(t, "u2", "bob@example.com"),
		}
		env.cfg.GraphClient = graphClient

		r, err := New(env.cfg)
		require.NoError(t, err)
		result, err := r.Reconcile(ctx, mdmsync.SyncModeFull)
		require.NoError(t, err)
		require.Equal(t, 2, result.ImportedUsers)

		users, err := listTeleportUsers(ctx, env.cfg.AccessPoint, env.cfg.SSOConnectorID)
		require.NoError(t, err)

		require.Len(t, users, 2)
	})

	t.Run("User account skipped on sanitization error", func(t *testing.T) {
		graphClient := newFakeGraphClient()
		env := NewEnv(t, graphClient, nil /* custom saml connector */)
		graphClient.users = []*models.User{
			entraUser(t, "u1", "al'ice@example.com"),
			entraUser(t, "u2", "bob@example.com"),
			entraUser(t, "u3", "carol@example.com"),
		}

		g1 := newEntraGroup(t, "g1", "apple")
		g2 := newEntraGroup(t, "g2", "banana")
		graphClient.groups = []*models.Group{g1, g2}

		graphClient.groupMembers = map[string][]models.GroupMember{
			"g1": {
				entraUser(t, "u1", "al'ice@example.com"),
				entraUser(t, "u2", "bob@example.com"),
				entraUser(t, "u3", "carol@example.com"),
			},
			"g2": {
				entraUser(t, "u2", "bob@example.com"),
				entraUser(t, "u1", "al'ice@example.com"),
				entraUser(t, "u3", "carol@example.com"),
			},
		}
		env.cfg.GraphClient = graphClient

		r, err := New(env.cfg)
		require.NoError(t, err)
		result, err := r.Reconcile(ctx, mdmsync.SyncModeFull)
		require.NoError(t, err)
		require.Equal(t, 2, result.ImportedUsers)
		require.Equal(t, 2, result.ImportedGroups)
		require.ErrorContains(t, result.ErrSkippedResources, "al'ice@example.com")

		users, err := listTeleportUsers(ctx, env.cfg.AccessPoint, env.cfg.SSOConnectorID)
		require.NoError(t, err)
		require.Len(t, users, 2)

		requireAccessListCount(t, env.aclSvc, 2)
		al1 := requireAccessListForEntraGroupExists(t, env.aclSvc, g1)
		al2 := requireAccessListForEntraGroupExists(t, env.aclSvc, g2)

		requireMembersCount(t, env.aclSvc, 4)
		requireMemberExists(t, env.aclSvc, al1, "bob@example.com")
		requireMemberExists(t, env.aclSvc, al1, "carol@example.com")
		requireMemberExists(t, env.aclSvc, al2, "bob@example.com")
		requireMemberExists(t, env.aclSvc, al2, "carol@example.com")
	})

	t.Run("Entra ID user conflicting with existing local user account is skipped", func(t *testing.T) {
		graphClient := newFakeGraphClient()
		env := NewEnv(t, graphClient, nil /* custom saml connector */)

		// create bob as local user
		bobTeleport, err := types.NewUser("bob@example.com")
		require.NoError(t, err)
		_, err = env.cfg.AccessPoint.CreateUser(ctx, bobTeleport)
		require.NoError(t, err)
		graphClient.users = []*models.User{
			entraUser(t, "u1", "alice@example.com"),
			entraUser(t, "u2", "bob@example.com"),
			entraUser(t, "u3", "carol@example.com"),
		}

		g1 := newEntraGroup(t, "g1", "apple")
		g2 := newEntraGroup(t, "g2", "banana")
		graphClient.groups = []*models.Group{g1, g2}
		graphClient.groupMembers = map[string][]models.GroupMember{
			"g1": {
				entraUser(t, "u1", "alice@example.com"),
				entraUser(t, "u2", "bob@example.com"),
				entraUser(t, "u3", "carol@example.com"),
			},
			"g2": {
				entraUser(t, "u2", "bob@example.com"),
				entraUser(t, "u3", "carol@example.com"),
			},
		}
		env.cfg.GraphClient = graphClient

		r, err := New(env.cfg)
		require.NoError(t, err)
		result, err := r.Reconcile(ctx, mdmsync.SyncModeFull)
		require.NoError(t, err)
		require.Equal(t, 2, result.ImportedUsers)
		require.Equal(t, 2, result.ImportedGroups)
		require.ErrorContains(t, result.ErrSkippedResources, "bob@example.com")

		users, err := listTeleportUsers(ctx, env.cfg.AccessPoint, env.cfg.SSOConnectorID)
		require.NoError(t, err)
		require.Len(t, users, 2)

		requireAccessListCount(t, env.aclSvc, 2)
		al1 := requireAccessListForEntraGroupExists(t, env.aclSvc, g1)
		al2 := requireAccessListForEntraGroupExists(t, env.aclSvc, g2)

		requireMemberExists(t, env.aclSvc, al1, "alice@example.com")
		requireMemberExists(t, env.aclSvc, al1, "carol@example.com")
		requireMemberExists(t, env.aclSvc, al2, "carol@example.com")

		requireMemberDoesNotExists(t, env.aclSvc, al1, "bob@example.com")
		requireMemberDoesNotExists(t, env.aclSvc, al2, "bob@example.com")
	})

	t.Run("SAML user skipped if its not created by the plugin or by the referenced connector", func(t *testing.T) {
		graphClient := newFakeGraphClient()
		env := NewEnv(t, graphClient, nil /* custom saml connector */)

		// create bob with different connector
		bobTeleport, err := types.NewUser("bob@example.com")
		require.NoError(t, err)
		bobTeleport.SetCreatedBy(types.CreatedBy{
			Connector: &types.ConnectorRef{
				Type: constants.SAML,
				ID:   "abc-ref",
			},
		})
		_, err = env.cfg.AccessPoint.CreateUser(ctx, bobTeleport)
		require.NoError(t, err)
		graphClient.users = []*models.User{
			entraUser(t, "u1", "alice@example.com"),
			entraUser(t, "u2", "bob@example.com"),
			entraUser(t, "u3", "carol@example.com"),
		}

		g1 := newEntraGroup(t, "g1", "apple")
		g2 := newEntraGroup(t, "g2", "banana")
		graphClient.groups = []*models.Group{g1, g2}
		graphClient.groupMembers = map[string][]models.GroupMember{
			"g1": {
				entraUser(t, "u1", "alice@example.com"),
				entraUser(t, "u2", "bob@example.com"),
				entraUser(t, "u3", "carol@example.com"),
			},
			"g2": {
				entraUser(t, "u2", "bob@example.com"),
				entraUser(t, "u3", "carol@example.com"),
			},
		}
		env.cfg.GraphClient = graphClient

		r, err := New(env.cfg)
		require.NoError(t, err)
		result, err := r.Reconcile(ctx, mdmsync.SyncModeFull)
		require.NoError(t, err)
		require.ErrorContains(t, result.ErrSkippedResources, "bob@example.com")
		require.ErrorContains(t, result.ErrSkippedResources, "Member IDs: u2")

		users, err := listTeleportUsers(ctx, env.cfg.AccessPoint, env.cfg.SSOConnectorID)
		require.NoError(t, err)
		require.Len(t, users, 2)

		requireAccessListCount(t, env.aclSvc, 2)
		al1 := requireAccessListForEntraGroupExists(t, env.aclSvc, g1)
		al2 := requireAccessListForEntraGroupExists(t, env.aclSvc, g2)

		requireMemberExists(t, env.aclSvc, al1, "alice@example.com")
		requireMemberExists(t, env.aclSvc, al1, "carol@example.com")
		requireMemberExists(t, env.aclSvc, al2, "carol@example.com")

		requireMemberDoesNotExists(t, env.aclSvc, al1, "bob@example.com")
		requireMemberDoesNotExists(t, env.aclSvc, al2, "bob@example.com")
	})

	t.Run("User account overwritten if it was created by the referenced connector", func(t *testing.T) {
		graphClient := newFakeGraphClient()
		env := NewEnv(t, graphClient, nil /* custom saml connector */)
		bobTeleport, err := types.NewUser("bob@example.com")
		require.NoError(t, err)
		bobTeleport.SetCreatedBy(types.CreatedBy{
			Connector: &types.ConnectorRef{
				Type: constants.SAML,
				ID:   ssoConnectorID,
			},
		})
		_, err = env.cfg.AccessPoint.CreateUser(ctx, bobTeleport)
		require.NoError(t, err)
		graphClient.users = []*models.User{
			entraUser(t, "u1", "alice@example.com"),
			entraUser(t, "u2", "bob@example.com"),
		}
		g1 := newEntraGroup(t, "g1", "apple")
		g2 := newEntraGroup(t, "g2", "banana")
		graphClient.groups = []*models.Group{g1, g2}
		graphClient.groupMembers = map[string][]models.GroupMember{
			"g1": {
				entraUser(t, "u1", "alice@example.com"),
				entraUser(t, "u2", "bob@example.com"),
			},
			"g2": {
				entraUser(t, "u1", "alice@example.com"),
				entraUser(t, "u2", "bob@example.com"),
			},
		}
		env.cfg.GraphClient = graphClient

		r, err := New(env.cfg)
		require.NoError(t, err)
		result, err := r.Reconcile(ctx, mdmsync.SyncModeFull)
		require.NoError(t, err)
		require.Equal(t, 2, result.ImportedUsers)
		require.Equal(t, 2, result.ImportedGroups)
		require.NoError(t, result.ErrSkippedResources)

		users, err := listTeleportUsers(ctx, env.cfg.AccessPoint, env.cfg.SSOConnectorID)
		require.NoError(t, err)

		require.Len(t, users, 2)
		require.Equal(t, types.OriginEntraID, users["bob@example.com"].Origin())
		requireAccessListCount(t, env.aclSvc, 2)
		al1 := requireAccessListForEntraGroupExists(t, env.aclSvc, g1)
		al2 := requireAccessListForEntraGroupExists(t, env.aclSvc, g2)

		requireMemberExists(t, env.aclSvc, al1, "alice@example.com")
		requireMemberExists(t, env.aclSvc, al1, "bob@example.com")
		requireMemberExists(t, env.aclSvc, al2, "alice@example.com")
		requireMemberExists(t, env.aclSvc, al2, "bob@example.com")
	})
}

func TestUserResourceEqualsInDeltaAndFullSync(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	const bobUsername = "bob@example.com"
	bob := &models.User{
		DirectoryObject: models.DirectoryObject{
			ID:          to.Ptr(msgraphtest.BobID),
			DisplayName: to.Ptr("Bob Builder"),
		},
		UserPrincipalName:        to.Ptr(bobUsername),
		Mail:                     to.Ptr(bobUsername),
		OnPremisesSAMAccountName: to.Ptr("Bob the Builder"),
		GivenName:                to.Ptr("Bob"),
		Surname:                  to.Ptr("Builder"),
	}

	assertBasicUserAttributes := func(t *testing.T, u types.User) {
		t.Helper()

		// Check traits matches.
		// First item expected to match from traits values.
		traits := u.GetTraits()
		require.Equal(t, bobUsername, traits[entraIDSAMLClaimName][0], "username")
		require.Equal(t, tenantID, traits[tenantIDClaim][0], "tenant ID")
		require.Equal(t, *bob.GetID(), traits[objectIdentifierClaim][0], "user object ID")
		require.Equal(t, *bob.DisplayName, traits[displayNameClaim][0], "display name")
		require.Equal(t, *bob.GivenName, traits[entraIDSAMLGivenName][0], "given name")
		require.Equal(t, *bob.Surname, traits[entraIDSAMLSurname][0], "surname")

		// Check labels matches.
		require.Equal(t, types.OriginEntraID, u.Origin(), "user origin")
		labels := u.GetAllLabels()
		require.Equal(t, *bob.GetID(), labels[types.EntraUniqueIDLabel], "Entra unique ID")
		require.Equal(t, tenantID, labels[types.EntraTenantIDLabel], "Entra tenant ID")
		require.Equal(t, *bob.UserPrincipalName, labels[types.EntraUPNLabel], "Entra UPN")
		require.Equal(t, *bob.OnPremisesSAMAccountName, labels[types.EntraSAMAccountNameLabel], "Entra SAM account name")

		isExternal, err := strconv.ParseBool(labels[types.TeleportInternalLabelPrefix+"entra-is-external"])
		require.NoError(t, err)
		require.False(t, isExternal, "entra-is-external label")

		// Check CreatedBy matches.
		require.Equal(t, constants.SAML, u.GetCreatedBy().Connector.Type, "user CreatedBy connector type")
		require.Equal(t, ssoConnectorID, u.GetCreatedBy().Connector.ID, "user CreatedBy connector ID")
		require.Equal(t, *bob.UserPrincipalName, u.GetCreatedBy().Connector.Identity, "user CreatedBy identity")
	}

	cmpIgnore := cmpopts.IgnoreFields(types.Metadata{}, "Revision")
	t.Run("with group memberships", func(t *testing.T) {
		// Setup test env with msgraphtest fake server.
		defaultStorage := msgraphtest.NewDefaultStorage()
		graphStorage := msgraphtest.NewStorage()
		graphStorage.Applications = defaultStorage.Applications
		// Create one Entra ID user Alice.
		graphStorage.Users = make(map[string]*models.User)
		graphStorage.Users[msgraphtest.BobID] = bob

		graphStorage.Groups = make(map[string]*models.Group)
		graphStorage.Groups[msgraphtest.Group1ID] = defaultStorage.Groups[msgraphtest.Group1ID]
		graphStorage.Groups[msgraphtest.Group2ID] = defaultStorage.Groups[msgraphtest.Group2ID]

		graphStorage.GroupMembers = make(map[string][]models.GroupMember)
		graphStorage.GroupMembers[msgraphtest.Group1ID] = []models.GroupMember{bob}
		graphStorage.GroupMembers[msgraphtest.Group2ID] = []models.GroupMember{bob}

		env := newFakeEnv(t, graphStorage)
		env.cfg.DeltaSyncEnabled = true
		r, err := New(env.cfg)
		require.NoError(t, err)

		// Initial full sync
		_, err = r.Reconcile(ctx, mdmsync.SyncModeFull)
		require.NoError(t, err)

		bobResourceAfterFullSync, err := env.identitySvc.GetUser(ctx, bobUsername, false)
		require.NoError(t, err)

		// Sanity check basic user attributes.
		assertBasicUserAttributes(t, bobResourceAfterFullSync)

		// Check role and traits.
		// Bob is member of Group1ID and Group1ID.
		expectedTraits := []string{msgraphtest.Group1ID, msgraphtest.Group2ID}
		require.ElementsMatch(t, expectedTraits, bobResourceAfterFullSync.GetTraits()[entraIDSAMLClaimGroups], "group traits")

		// access and editor role grant is baked in newSAMLConnector that matches with
		// msgraphtest Group1ID and Group2ID.
		expectedRoles := []string{"access", "editor"}
		require.ElementsMatch(t, expectedRoles, bobResourceAfterFullSync.GetRoles(), "roles differ")

		// Run reconciler mimicking delta sync.
		_, err = r.Reconcile(ctx, mdmsync.SyncModePartial)
		require.NoError(t, err)
		bobResourceAfterDeltaSync, err := env.identitySvc.GetUser(ctx, bobUsername, false)
		require.NoError(t, err, "expected Bob account in Teleport backend")

		require.Empty(t, cmp.Diff(bobResourceAfterFullSync, bobResourceAfterDeltaSync, cmpIgnore))
	})

	t.Run("without group memberships", func(t *testing.T) {
		// Setup test env with msgraphtest fake server.
		graphStorage := msgraphtest.NewStorage()
		graphStorage.Applications = msgraphtest.NewDefaultStorage().Applications
		// Create one Entra ID user Alice.
		graphStorage.Users = make(map[string]*models.User)

		graphStorage.Users[msgraphtest.BobID] = bob

		env := newFakeEnv(t, graphStorage)
		env.cfg.DeltaSyncEnabled = true
		r, err := New(env.cfg)
		require.NoError(t, err)

		// Initial full sync
		_, err = r.Reconcile(ctx, mdmsync.SyncModeFull)
		require.NoError(t, err)

		bobResourceAfterFullSync, err := env.identitySvc.GetUser(ctx, bobUsername, false)
		require.NoError(t, err, "expected Bob account in Teleport backend")

		// Sanity check basic user attributes.
		assertBasicUserAttributes(t, bobResourceAfterFullSync)

		// entraIDSAMLClaimGroups should be nil because user has zero group membership.
		require.Nil(t, bobResourceAfterFullSync.GetTraits()[entraIDSAMLClaimGroups], "group traits")
		// No role grants because user has zero group membership.
		require.Empty(t, bobResourceAfterFullSync.GetRoles(), "role grants")

		// Run reconciler mimicking delta sync.
		_, err = r.Reconcile(ctx, mdmsync.SyncModePartial)
		require.NoError(t, err)
		bobResourceAfterDeltaSync, err := env.identitySvc.GetUser(ctx, bobUsername, false)
		require.NoError(t, err, "expected Bob account in Teleport backend")

		require.Empty(t, cmp.Diff(bobResourceAfterFullSync, bobResourceAfterDeltaSync, cmpIgnore))
	})
}

func TestGroupFilters(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	graphClient := newFakeGraphClient()
	env := NewEnv(t, graphClient, nil /* custom saml connector */)

	testCases := []struct {
		name        string
		entraGroups []*models.Group
		filters     filter.Filters
		expected    []*models.Group
	}{
		{
			name: "Filter by ID",
			entraGroups: []*models.Group{
				newEntraGroup(t, "1", "apple"),
				newEntraGroup(t, "2", "banana"),
			},
			filters: filter.Filters{
				&types.PluginSyncFilter{Include: &types.PluginSyncFilter_Id{Id: "2"}},
			},
			expected: []*models.Group{newEntraGroup(t, "2", "banana")},
		},
		{
			name: "Filter by Name",
			entraGroups: []*models.Group{
				newEntraGroup(t, "1", "apple"),
				newEntraGroup(t, "2", "admin"),
				newEntraGroup(t, "3", "banana"),
			},
			filters: filter.Filters{
				&types.PluginSyncFilter{Include: &types.PluginSyncFilter_NameRegex{NameRegex: "a*"}},
			},
			expected: []*models.Group{
				newEntraGroup(t, "1", "apple"),
				newEntraGroup(t, "2", "admin"),
			},
		},
		{
			name: "Exclude All",
			entraGroups: []*models.Group{
				newEntraGroup(t, "1", "apple"),
				newEntraGroup(t, "2", "admin"),
				newEntraGroup(t, "3", "banana"),
			},
			filters: filter.Filters{
				&types.PluginSyncFilter{Include: &types.PluginSyncFilter_NameRegex{NameRegex: "teleport.internal/exclude_all"}},
			},
			expected: nil,
		},
		{
			name: "No Filters (matches all)",
			entraGroups: []*models.Group{
				newEntraGroup(t, "1", "apple"),
				newEntraGroup(t, "2", "admin"),
				newEntraGroup(t, "3", "banana"),
			},
			filters: nil,
			expected: []*models.Group{
				newEntraGroup(t, "1", "apple"),
				newEntraGroup(t, "2", "admin"),
				newEntraGroup(t, "3", "banana"),
			},
		},
		{
			name: "Multiple Filters",
			entraGroups: []*models.Group{
				newEntraGroup(t, "1", "apple"),
				newEntraGroup(t, "2", "admin"),
				newEntraGroup(t, "3", "banana"),
				newEntraGroup(t, "4", "carrot"),
			},
			filters: filter.Filters{
				&types.PluginSyncFilter{Include: &types.PluginSyncFilter_Id{Id: "2"}},
				&types.PluginSyncFilter{Include: &types.PluginSyncFilter_NameRegex{NameRegex: "a*"}},
				&types.PluginSyncFilter{Include: &types.PluginSyncFilter_Id{Id: "4"}},
			},
			expected: []*models.Group{
				newEntraGroup(t, "1", "apple"),
				newEntraGroup(t, "2", "admin"),
				newEntraGroup(t, "4", "carrot"),
			},
		},
		{
			name: "Exclude by ID",
			entraGroups: []*models.Group{
				newEntraGroup(t, "1", "apple"),
				newEntraGroup(t, "2", "banana"),
			},
			filters: filter.Filters{
				&types.PluginSyncFilter{Exclude: &types.PluginSyncFilter_ExcludeId{ExcludeId: "2"}},
			},
			expected: []*models.Group{newEntraGroup(t, "1", "apple")},
		},
		{
			name: "Exclude by NameRegex",
			entraGroups: []*models.Group{
				newEntraGroup(t, "1", "apple"),
				newEntraGroup(t, "2", "admin"),
				newEntraGroup(t, "3", "banana"),
			},
			filters: filter.Filters{
				&types.PluginSyncFilter{Exclude: &types.PluginSyncFilter_ExcludeNameRegex{ExcludeNameRegex: "a*"}},
			},
			expected: []*models.Group{newEntraGroup(t, "3", "banana")},
		},
		{
			name: "Include and Exclude - exclude wins",
			entraGroups: []*models.Group{
				newEntraGroup(t, "1", "apple"),
				newEntraGroup(t, "2", "admin"),
				newEntraGroup(t, "3", "banana"),
				newEntraGroup(t, "4", "carrot"),
			},
			filters: filter.Filters{
				&types.PluginSyncFilter{Include: &types.PluginSyncFilter_NameRegex{NameRegex: "*"}},
				&types.PluginSyncFilter{Exclude: &types.PluginSyncFilter_ExcludeId{ExcludeId: "1"}},
			},
			expected: []*models.Group{
				newEntraGroup(t, "2", "admin"),
				newEntraGroup(t, "3", "banana"),
				newEntraGroup(t, "4", "carrot"),
			},
		},
		{
			name: "Include and Exclude - matching include/exclude regexp",
			entraGroups: []*models.Group{
				newEntraGroup(t, "1", "apple"),
				newEntraGroup(t, "2", "admin"),
				newEntraGroup(t, "3", "banana"),
				newEntraGroup(t, "4", "carrot"),
			},
			filters: filter.Filters{
				&types.PluginSyncFilter{Exclude: &types.PluginSyncFilter_ExcludeNameRegex{ExcludeNameRegex: "a*"}},
				&types.PluginSyncFilter{Include: &types.PluginSyncFilter_NameRegex{NameRegex: "a*"}},
				&types.PluginSyncFilter{Include: &types.PluginSyncFilter_NameRegex{NameRegex: "b*"}},
			},
			expected: []*models.Group{newEntraGroup(t, "3", "banana")},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			graphClient.groups = tc.entraGroups
			env.cfg.GroupsFilter = tc.filters
			env.cfg.GraphClient = graphClient

			r, err := New(env.cfg)
			require.NoError(t, err)

			result, err := r.Reconcile(ctx, mdmsync.SyncModeFull)
			require.NoError(t, err)
			require.NoError(t, result.ErrSkippedResources)

			requireAccessListCount(t, env.aclSvc, len(tc.expected))
			for _, g := range tc.expected {
				requireAccessListForEntraGroupExists(t, env.aclSvc, g)
			}
		})
	}
}

func TestInvalidGroupIsSkipped(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	graphClient := newFakeGraphClient()
	env := NewEnv(t, graphClient, nil /* custom saml connector */)

	testCases := []struct {
		name           string
		entraGroups    []*models.Group
		expectedGroups []*models.Group
	}{
		{
			name: "Empty group",
			entraGroups: []*models.Group{
				newEntraGroup(t, "1", "apple"),
				// this invalid group should not prevent the group below to be synced.
				{},
				newEntraGroup(t, "2", "banana"),
			},
			expectedGroups: []*models.Group{
				newEntraGroup(t, "1", "apple"),
				newEntraGroup(t, "2", "banana"),
			},
		},
		{
			name: "ID Missing",
			entraGroups: []*models.Group{
				newEntraGroup(t, "1", "apple"),
				// this invalid group should not prevent the group below to be synced.
				{
					DirectoryObject: models.DirectoryObject{
						DisplayName: to.Ptr("carrot"),
					},
				},
				newEntraGroup(t, "2", "banana"),
			},
			expectedGroups: []*models.Group{
				newEntraGroup(t, "1", "apple"),
				newEntraGroup(t, "2", "banana"),
			},
		},
		{
			name: "DisplayName missing",
			entraGroups: []*models.Group{
				newEntraGroup(t, "1", "apple"),
				// this invalid group should not prevent the group below to be synced.
				{
					DirectoryObject: models.DirectoryObject{
						ID: to.Ptr("3"),
					},
				},
				newEntraGroup(t, "2", "banana"),
			},
			expectedGroups: []*models.Group{
				newEntraGroup(t, "1", "apple"),
				newEntraGroup(t, "2", "banana"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			graphClient.groups = tc.entraGroups
			env.cfg.GraphClient = graphClient

			r, err := New(env.cfg)
			require.NoError(t, err)

			result, err := r.Reconcile(ctx, mdmsync.SyncModeFull)
			require.NoError(t, err)
			require.ErrorContains(t, result.ErrSkippedResources, "have a non-empty")

			requireAccessListCount(t, env.aclSvc, len(tc.expectedGroups))
			for _, g := range tc.expectedGroups {
				requireAccessListForEntraGroupExists(t, env.aclSvc, g)
			}
		})
	}
}

// Tests a scenario where an unknown filter should
// discard filters altogether and instead only reconcile
// items that are already synced to Teleport.
func TestUnknownFilter(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	graphClient := newFakeGraphClient()
	env := NewEnv(t, graphClient, nil /* custom saml connector */)

	// Start with 2 users, 4 groups, 3 group members,
	// and reconcile without any filters.
	graphClient.users = []*models.User{
		entraUser(t, "u1", "alice@example.com"),
		entraUser(t, "u2", "bob@example.com"),
	}
	g1 := newEntraGroup(t, "g1", "apple")
	g2 := newEntraGroup(t, "g2", "admin")
	g3 := newEntraGroup(t, "g3", "banana")
	g4 := newEntraGroup(t, "g4", "carrot")
	entraGroups := []*models.Group{g1, g2, g3, g4}
	graphClient.groups = entraGroups
	members := map[string][]models.GroupMember{
		"g1": {entraUser(t, "u1", "alice@example.com")},
		"g2": {entraUser(t, "u1", "alice@example.com")},
		"g3": {entraUser(t, "u2", "bob@example.com")},
	}
	graphClient.groupMembers = members
	env.cfg.GraphClient = graphClient

	// reconcile
	r, err := New(env.cfg)
	require.NoError(t, err)

	result, err := r.Reconcile(ctx, mdmsync.SyncModeFull)
	require.NoError(t, err)
	require.Equal(t, 4, result.ImportedGroups)

	requireAccessListCount(t, env.aclSvc, 4)
	al1 := requireAccessListForEntraGroupExists(t, env.aclSvc, g1)
	al2 := requireAccessListForEntraGroupExists(t, env.aclSvc, g2)
	al3 := requireAccessListForEntraGroupExists(t, env.aclSvc, g3)
	_ = requireAccessListForEntraGroupExists(t, env.aclSvc, g4)

	requireMembersCount(t, env.aclSvc, 3)
	requireMemberExists(t, env.aclSvc, al1, "alice@example.com")
	requireMemberExists(t, env.aclSvc, al2, "alice@example.com")
	requireMemberExists(t, env.aclSvc, al3, "bob@example.com")

	// Now update the filter with an unknown filter type
	// and test reconciliation only happens for groups
	// that are already synced to Teleport before the
	// introduction of an "unknown" filter.

	// Of the 4 test groups we started with, mock that
	// group g2 is deleted, but two new groups g5 and g6
	// are added in Entra ID.
	g5 := newEntraGroup(t, "g5", "drum")
	g6 := newEntraGroup(t, "g6", "eagle")
	newGroup := []*models.Group{g1, g3, g4, g5, g6}
	graphClient.groups = newGroup
	// g1 group gets one additional member
	members["g1"] = []models.GroupMember{entraUser(t, "u1", "alice@example.com"), entraUser(t, "u2", "bob@example.com")}
	graphClient.groupMembers = members

	type unsupportedFilterType struct {
		types.PluginSyncFilter_Id
	}
	env.cfg.GroupsFilter = []*types.PluginSyncFilter{
		{Include: &unsupportedFilterType{}},
	}

	// reconcile
	r, err = New(env.cfg)
	require.NoError(t, err)

	result, err = r.Reconcile(ctx, mdmsync.SyncModeFull)
	require.NoError(t, err)
	require.Equal(t, 3, result.ImportedGroups)

	// If the group was deleted in entra, it must be deleted in Teleport.
	// If group member was added to already-synced group, that must be reflected.
	// As a result: g2 should be deleted, g5 and g6 should not be added,
	// a new member should be added to group g1.

	requireAccessListCount(t, env.aclSvc, 3)
	al1 = requireAccessListForEntraGroupExists(t, env.aclSvc, g1)
	al3 = requireAccessListForEntraGroupExists(t, env.aclSvc, g3)
	_ = requireAccessListForEntraGroupExists(t, env.aclSvc, g4)

	requireMembersCount(t, env.aclSvc, 3)
	requireMemberExists(t, env.aclSvc, al1, "alice@example.com")
	requireMemberExists(t, env.aclSvc, al1, "bob@example.com")
	requireMemberExists(t, env.aclSvc, al3, "bob@example.com")
}

func TestNestedMembership(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	graphClient := newFakeGraphClient()
	env := NewEnv(t, graphClient, nil /* custom saml connector */)

	graphClient.users = []*models.User{
		entraUser(t, "u1", "alice@example.com"),
		entraUser(t, "u2", "bob@example.com"),
	}
	g1 := newEntraGroup(t, "g1", "apple")
	g2 := newEntraGroup(t, "g2", "admin")
	g3 := newEntraGroup(t, "g3", "banana")
	g4 := newEntraGroup(t, "g4", "carrot")
	entraGroups := []*models.Group{g1, g2, g3, g4}
	graphClient.groups = entraGroups
	graphClient.groupMembers = map[string][]models.GroupMember{
		"g1": {g2},
		"g2": {g3},
		"g3": {g4},
	}
	env.cfg.GraphClient = graphClient

	r, err := New(env.cfg)
	require.NoError(t, err)
	_, err = r.Reconcile(ctx, mdmsync.SyncModeFull)
	require.NoError(t, err)

	requireAccessListCount(t, env.aclSvc, 4)
	al1 := requireAccessListForEntraGroupExists(t, env.aclSvc, g1)
	al2 := requireAccessListForEntraGroupExists(t, env.aclSvc, g2)
	al3 := requireAccessListForEntraGroupExists(t, env.aclSvc, g3)
	al4 := requireAccessListForEntraGroupExists(t, env.aclSvc, g4)

	requireMembersCount(t, env.aclSvc, 3)
	requireMemberExists(t, env.aclSvc, al1, al2.GetName())
	requireMemberExists(t, env.aclSvc, al2, al3.GetName())
	requireMemberExists(t, env.aclSvc, al3, al4.GetName())

	// Let's create a cycle, by g4, going back to g1 and expect a reconciliation error.

	graphClient.groupMembers = map[string][]models.GroupMember{
		"g1": {g2},
		"g2": {g3},
		"g3": {g4},
		"g4": {g1},
	}

	_ /* result */, err = r.Reconcile(ctx, mdmsync.SyncModeFull)
	require.ErrorContains(t, err, "is already included as a Member or Owner in")
}

func TestRestoreDeltaLink(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Setup test env with msgraphtest fake server.
	graphStorage := msgraphtest.NewStorage()
	graphStorage.Applications = msgraphtest.NewDefaultStorage().Applications
	// Create one Entra ID user Alice.
	graphStorage.Users = make(map[string]*models.User)
	const aliceUsername = "alice@example.com"
	alice := entraUser(t, msgraphtest.AliceID, aliceUsername)
	graphStorage.Users[msgraphtest.AliceID] = alice

	env := newFakeEnv(t, graphStorage)
	env.cfg.DeltaSyncEnabled = true
	r, err := New(env.cfg)
	require.NoError(t, err)

	// Initial full sync, should setup delta sync as well.
	_, err = r.Reconcile(ctx, mdmsync.SyncModeFull)
	require.NoError(t, err)
	user, err := env.identitySvc.GetUser(ctx, aliceUsername, false)
	require.NoError(t, err)
	require.Equal(t, aliceUsername, user.GetName())

	// Read current delta link for user and group.
	oldUserDeltaLink := r.graphClient.deltaStore.Get(usersDeltaEndpoint)
	require.NotEmpty(t, oldUserDeltaLink)
	oldGroupDeltaLink := r.graphClient.deltaStore.Get(groupsDeltaEndpoint)
	require.NotEmpty(t, oldGroupDeltaLink)

	// Create a delta change by deleting Alice.
	env.fakeGraphServer.DeleteUsers([]string{msgraphtest.AliceID})

	// Fake a transient Teleport issue.
	errTransientIssue := errors.New("transient Teleport error")
	r.accessPoint = fakeDeleteUserAccessPoint{
		accessPoint: r.accessPoint,
		deleteUser: func(ctx context.Context, user string) error {
			// Check delta link was updated because delta API was queried successfully.
			require.NotEqual(t, oldUserDeltaLink, r.graphClient.deltaStore.Get(usersDeltaEndpoint))
			require.NotEqual(t, oldGroupDeltaLink, r.graphClient.deltaStore.Get(groupsDeltaEndpoint))
			return errTransientIssue
		},
	}

	// Delta sync with transient Teleport error.
	_, err = r.Reconcile(ctx, mdmsync.SyncModePartial)
	require.ErrorIs(t, err, errTransientIssue)

	// Old delta link is preserved.
	require.Equal(t, oldUserDeltaLink, r.graphClient.deltaStore.Get(usersDeltaEndpoint))
	require.Equal(t, oldGroupDeltaLink, r.graphClient.deltaStore.Get(groupsDeltaEndpoint))
}

type directoryReconcilerEnv struct {
	cfg             Config
	identitySvc     *local.IdentityService
	aclSvc          *local.AccessListService
	fakeGraphServer *msgraphtest.Server
}

const ssoConnectorID = "my-sso-connector"

func NewEnv(t *testing.T, graphClient *fakeGraphClient, connector types.SAMLConnector) directoryReconcilerEnv {
	mem, err := memory.New(memory.Config{})
	require.NoError(t, err)
	bk := backend.NewSanitizer(mem)
	identitySvc, err := local.NewIdentityService(bk)
	require.NoError(t, err)
	alSvc, err := local.NewAccessListServiceV2(local.AccessListServiceConfig{
		Backend: bk,
		Modules: modulestest.EnterpriseModules(),
	})
	require.NoError(t, err)

	samlService, err := local.NewIdentityService(bk)
	require.NoError(t, err)
	if connector == nil {
		connector = newSAMLConnector(t, ssoConnectorID, uuid.NewString(), uuid.NewString())
	}
	_, err = samlService.CreateSAMLConnector(t.Context(), connector)
	require.NoError(t, err)

	applicationID := uuid.NewString()
	application := &models.Application{
		AppID: to.Ptr(applicationID),
		DirectoryObject: models.DirectoryObject{
			DisplayName: to.Ptr("My Application"),
			ID:          to.Ptr(uuid.NewString()),
		},
		OptionalClaims: &models.OptionalClaims{
			SAML2Token: []models.OptionalClaim{
				{
					Name: to.Ptr("group"),
				},
			},
		},
	}
	graphClient.applications = append(graphClient.applications, application)

	tenantID := uuid.NewString()
	defaultOwners := []accesslist.Owner{
		{Name: "admin", MembershipKind: accesslist.MembershipKindUser, IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String()},
		{Name: "reviewer", MembershipKind: accesslist.MembershipKindUser, IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String()},
	}

	type ap struct {
		accessListAccessPoint
		userAccessPoint
		connectorAccessPoint
	}

	cfg := Config{
		Clock:       clockwork.NewRealClock(),
		Logger:      logtest.NewLogger(),
		GraphClient: graphClient,
		AccessPoint: ap{
			accessListAccessPoint: alSvc,
			userAccessPoint:       identitySvc,
			connectorAccessPoint:  samlService,
		},
		DefaultOwners:  defaultOwners,
		SSOConnectorID: connector.GetName(),
		TenantID:       tenantID,
		EntraAppID:     applicationID,
	}
	require.NoError(t, cfg.Validate())

	return directoryReconcilerEnv{
		cfg:         cfg,
		identitySvc: identitySvc,
		aclSvc:      alSvc,
	}
}

// newFakeEnv creates new [directoryReconcilerEnv] based on msgraphtest fake server.
func newFakeEnv(t *testing.T, storage *msgraphtest.Storage) directoryReconcilerEnv {
	mem, err := memory.New(memory.Config{})
	require.NoError(t, err)
	bk := backend.NewSanitizer(mem)
	identitySvc, err := local.NewIdentityService(bk)
	require.NoError(t, err)
	alSvc, err := local.NewAccessListServiceV2(local.AccessListServiceConfig{
		Backend: bk,
		Modules: modulestest.EnterpriseModules(),
	})
	require.NoError(t, err)

	samlService, err := local.NewIdentityService(bk)
	require.NoError(t, err)

	connector := newSAMLConnector(t, ssoConnectorID, msgraphtest.Group1ID, msgraphtest.Group2ID)
	_, err = samlService.CreateSAMLConnector(t.Context(), connector)
	require.NoError(t, err)

	defaultOwners := []accesslist.Owner{
		{Name: "admin", MembershipKind: accesslist.MembershipKindUser, IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String()},
	}

	type ap struct {
		accessListAccessPoint
		userAccessPoint
		connectorAccessPoint
	}

	// Set up fake graph API server.
	fakeServer := msgraphtest.NewServer(msgraphtest.WithStorage(storage))
	t.Cleanup(fakeServer.TLSServer.Close)
	httpClient := &http.Client{
		Transport: &msgraphtest.RewriteTransport{
			Base: fakeServer.TLSServer.Client().Transport,
			URL:  mustParseURL(t, fakeServer.TLSServer.URL),
		},
	}
	graphClient, err := msgraph.NewClient(msgraph.Config{
		HTTPClient:    httpClient,
		TokenProvider: &fakeTokenProvider{},
	})
	require.NoError(t, err)

	cfg := Config{
		Clock:       clockwork.NewRealClock(),
		Logger:      logtest.NewLogger(),
		GraphClient: graphClient,
		AccessPoint: ap{
			accessListAccessPoint: alSvc,
			userAccessPoint:       identitySvc,
			connectorAccessPoint:  samlService,
		},
		DefaultOwners:  defaultOwners,
		SSOConnectorID: connector.GetName(),
		TenantID:       tenantID,
		EntraAppID:     msgraphtest.App1ID,
	}
	require.NoError(t, cfg.Validate())

	return directoryReconcilerEnv{
		cfg:             cfg,
		identitySvc:     identitySvc,
		aclSvc:          alSvc,
		fakeGraphServer: fakeServer,
	}
}

func newSAMLConnector(t *testing.T, connectorID, group1, group2 string) types.SAMLConnector {
	t.Helper()
	connector, err := types.NewSAMLConnector(
		connectorID,
		types.SAMLConnectorSpecV2{
			AssertionConsumerService: "http://localhost:65535/acs", // not called
			Issuer:                   "test",
			SSO:                      "https://localhost:65535/sso", // not called
			AttributesToRoles: []types.AttributeMapping{
				{Name: "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups", Value: group1, Roles: []string{"access"}},
				{Name: "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups", Value: group2, Roles: []string{"editor"}},
			},
		})
	require.NoError(t, err)
	return connector
}

func newEntraGroup(t *testing.T, id, name string) *models.Group {
	t.Helper()
	return &models.Group{
		DirectoryObject: models.DirectoryObject{
			ID:          to.Ptr(id),
			DisplayName: to.Ptr(name),
		},
	}
}

func entraUser(t *testing.T, id, mail string) *models.User {
	t.Helper()
	return &models.User{
		DirectoryObject: models.DirectoryObject{
			ID: to.Ptr(id),
		},
		UserPrincipalName: to.Ptr(mail),
		Mail:              to.Ptr(mail),
	}
}

func requireAccessListCount(t *testing.T, srv services.AccessListsGetter, cnt int) {
	t.Helper()
	lists, err := srv.GetAccessLists(t.Context())
	require.NoError(t, err)
	require.Len(t, lists, cnt)
}

func requireAccessListForEntraGroupExists(t *testing.T, srv services.AccessListsGetter, g *models.Group) *accesslist.AccessList {
	t.Helper()

	lists, err := srv.GetAccessLists(t.Context())
	require.NoError(t, err)

	var found []*accesslist.AccessList
	for _, al := range lists {
		uid := al.GetAllLabels()[types.EntraUniqueIDLabel]
		displayName := al.GetAllLabels()[types.EntraDisplayNameLabel]
		if *g.ID == uid && *g.DisplayName == displayName {
			found = append(found, al)
		}
	}
	require.Len(t, found, 1)
	return found[0]
}

func requireMembersCount(t *testing.T, srv services.AccessListsGetter, cnt int) {
	t.Helper()
	actualCnt := 0
	for _, err := range clientutils.Resources(t.Context(), srv.ListAllAccessListMembers) {
		require.NoError(t, err)
		actualCnt++
	}
	require.Equal(t, cnt, actualCnt)
}

func requireMemberExists(t *testing.T, srv services.AccessListsGetter, accessList *accesslist.AccessList, member string) {
	t.Helper()
	_, err := srv.GetAccessListMember(t.Context(), accessList.GetName(), member)
	require.NoError(t, err)
}

func requireMemberDoesNotExists(t *testing.T, srv services.AccessListsGetter, accessList *accesslist.AccessList, member string) {
	t.Helper()
	_, err := srv.GetAccessListMember(t.Context(), accessList.GetName(), member)
	require.True(t, trace.IsNotFound(err))
}

type fakeDeleteUserAccessPoint struct {
	accessPoint
	deleteUser func(context.Context, string) error
}

func (a fakeDeleteUserAccessPoint) DeleteUser(ctx context.Context, user string) error {
	return a.deleteUser(ctx, user)
}

func mustParseURL(t *testing.T, in string) *url.URL {
	t.Helper()
	url, err := url.Parse(in)
	require.NoError(t, err)
	require.Equal(t, "https", url.Scheme, "expected URL with https scheme")
	return url
}

type fakeTokenProvider struct {
	mu    sync.Mutex
	token string
}

func (t *fakeTokenProvider) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.token == "" {
		t.token = uuid.NewString()
	}

	return azcore.AccessToken{
		Token: t.token,
	}, nil
}
