package directory

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"net/url"
	"path"
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
	"pgregory.net/rapid"

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
	libutils "github.com/gravitational/teleport/lib/utils"
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

	nameResolver := aclNameResolver{
		namesByID: make(aclNamesByGroupID), // fresh map to force new acl names.
	}
	_, teamCTeleport, err := convertGroup(teamCEntra, env.cfg.TenantID, env.cfg.DefaultOwners, nameResolver)
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

		subgroupAccessList := requireAccessListForEntraGroupExists(t, env.aclSvc, subgroup)
		require.Equal(t, subgroupAccessList.GetName(), subGroupMember.GetName())
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

		// Mimic user deletion.
		graphClient.users = []*models.User{
			entraUser(t, "u1", "alice@example.com"),
		}

		result, err = r.Reconcile(ctx, mdmsync.SyncModeFull)
		require.NoError(t, err)
		require.Equal(t, 1, result.ImportedUsers)
		users, err = listTeleportUsers(ctx, env.cfg.AccessPoint, env.cfg.SSOConnectorID)
		require.NoError(t, err)

		require.Len(t, users, 1)
		require.Nil(t, users["bob@example.com"])
		al1 = requireAccessListForEntraGroupExists(t, env.aclSvc, g1)
		_, err = env.aclSvc.GetAccessListMember(t.Context(), al1.GetName(), "bob@example.com")
		require.True(t, trace.IsNotFound(err))
	})

	t.Run("Conflicting users are skipped safely", func(t *testing.T) {
		graphClient := newFakeGraphClient()
		env := NewEnv(t, graphClient, nil /* custom saml connector */)

		// Users count less than 1000 are sequentially processed.
		// 1500 users should reliably trigger concurrency error.
		const userCount = 1500
		conflictingUsernames := make([]string, 0, userCount)
		graphClient.users = make([]*models.User, 0, userCount)

		for i := range userCount {
			username := fmt.Sprintf("conflict-%s@example.com", strconv.Itoa(i))
			conflictingUsernames = append(conflictingUsernames, username)

			// Create local user.
			localUser, err := types.NewUser(username)
			require.NoError(t, err)
			_, err = env.identitySvc.CreateUser(ctx, localUser)
			require.NoError(t, err)

			// Add Entra users with the same username as local users.
			graphClient.users = append(
				graphClient.users,
				entraUser(t, "u"+strconv.Itoa(i), username),
			)
		}

		// Reconcile.
		r, err := New(env.cfg)
		require.NoError(t, err)
		result, err := r.Reconcile(ctx, mdmsync.SyncModeFull)
		require.NoError(t, err) // Proves the reconciler ran without any race error.

		// Zero users imported due to conflict.
		require.Equal(t, 0, result.ImportedUsers)

		for _, username := range conflictingUsernames {
			// Proves that conflicting users were collected in a concurrency safe manner.
			require.ErrorContains(t, result.ErrSkippedResources, username)
		}
	})
}

func TestDeltaUserSyncConvertsSSOUser(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Bob is SSO user created by the connector referenced by the plugin.
	createBobSSOUser := func(t *testing.T, ssoConnectorID string) types.User {
		t.Helper()

		// Bob is SSO user created by the connector referenced by the plugin.
		bobTeleport, err := types.NewUser("bob@example.com")
		require.NoError(t, err)
		bobTeleport.SetCreatedBy(types.CreatedBy{
			Connector: &types.ConnectorRef{
				Type: constants.SAML,
				ID:   ssoConnectorID,
			},
		})
		return bobTeleport
	}

	// Simulate that the SSO user was immediately discovered in the delta
	// sync and was properly converted as Entra ID user.
	t.Run("immediate Graph user discovery", func(t *testing.T) {
		graphStorage := msgraphtest.NewStorage()
		defaultStorage := msgraphtest.NewDefaultStorage()
		graphStorage.Applications = defaultStorage.Applications
		// Start with zero users.
		graphStorage.Users = make(map[string]*models.User)

		env := newFakeEnv(t, withFakeEnvStorage(graphStorage))
		env.cfg.DeltaSyncEnabled = true

		r, err := New(env.cfg)
		require.NoError(t, err)

		// First full sync.
		result, err := r.Reconcile(ctx, mdmsync.SyncModeFull)
		require.NoError(t, err)
		require.Equal(t, 0, result.ImportedUsers)

		bobTeleport := createBobSSOUser(t, env.cfg.SSOConnectorID)
		_, err = env.cfg.AccessPoint.CreateUser(ctx, bobTeleport)
		require.NoError(t, err)

		// Create a delta diff for Alice and Bob user.
		// In this case, Alice should be created in Teleport, Bob's SSO user account
		// should be promoted as the Entra ID user.
		env.fakeGraphServer.SetUsers([]*models.User{defaultStorage.Users[msgraphtest.AliceID]})
		env.fakeGraphServer.SetUsers([]*models.User{defaultStorage.Users[msgraphtest.BobID]})

		result, err = r.Reconcile(ctx, mdmsync.SyncModePartial)
		require.NoError(t, err)
		// Both Alice and Bob imported.
		require.Equal(t, 2, result.ImportedUsers)

		users, err := listTeleportUsers(ctx, env.cfg.AccessPoint, env.cfg.SSOConnectorID)
		require.NoError(t, err)

		require.Len(t, users, 2)
		require.NotNil(t, users["alice@example.com"])
		require.NotNil(t, users["bob@example.com"])

		users, err = listTeleportUsers(ctx, env.cfg.AccessPoint, env.cfg.SSOConnectorID)
		require.NoError(t, err)

		// Counts for Alice and Bob.
		require.Len(t, users, 2)
		require.Equal(t, types.OriginEntraID, users["alice@example.com"].Origin(), "Alice expected to have Entra ID origin")
		require.Equal(t, types.OriginEntraID, users["bob@example.com"].Origin(), "Bob expected to be overwritten")

		env.fakeGraphServer.DeleteUsers([]string{msgraphtest.BobID})
		result, err = r.Reconcile(ctx, mdmsync.SyncModePartial)
		require.NoError(t, err)

		// Only Alice remains and Bob was deleted.
		require.Equal(t, 1, result.ImportedUsers)
		users, err = listTeleportUsers(ctx, env.cfg.AccessPoint, env.cfg.SSOConnectorID)
		require.NoError(t, err)
		require.Len(t, users, 1)
	})

	// Simulate that there was a delay when SSO user was discovered from the
	// Graph API and in the meantime, the SSO user account must be preserved.
	t.Run("delayed Graph user discovery", func(t *testing.T) {
		graphStorage := msgraphtest.NewStorage()
		defaultStorage := msgraphtest.NewDefaultStorage()
		graphStorage.Applications = defaultStorage.Applications
		// Start with zero users.
		graphStorage.Users = make(map[string]*models.User)

		env := newFakeEnv(t, withFakeEnvStorage(graphStorage))
		env.cfg.DeltaSyncEnabled = true

		r, err := New(env.cfg)
		require.NoError(t, err)

		// First full sync.
		result, err := r.Reconcile(ctx, mdmsync.SyncModeFull)
		require.NoError(t, err)
		require.Equal(t, 0, result.ImportedUsers)

		bobTeleport := createBobSSOUser(t, env.cfg.SSOConnectorID)
		_, err = env.cfg.AccessPoint.CreateUser(ctx, bobTeleport)
		require.NoError(t, err)

		// Create a delta diff for Alice user.
		// In this case, Alice should be created in Teleport, Bob's account in Teleport
		// should be preserved.
		env.fakeGraphServer.SetUsers([]*models.User{defaultStorage.Users[msgraphtest.AliceID]})

		result, err = r.Reconcile(ctx, mdmsync.SyncModePartial)
		require.NoError(t, err)
		// Only Alice is imported.
		require.Equal(t, 1, result.ImportedUsers)

		users, err := listTeleportUsers(ctx, env.cfg.AccessPoint, env.cfg.SSOConnectorID)
		require.NoError(t, err)

		require.Len(t, users, 2)
		require.NotNil(t, users["alice@example.com"]) // new user account.
		require.NotNil(t, users["bob@example.com"])   // existing SSO user account.

		// Create a delta diff now to simulate that when the users are discovered
		// from the delta API, they should not properly handled and overwritten.
		env.fakeGraphServer.SetUsers([]*models.User{defaultStorage.Users[msgraphtest.BobID]})

		result, err = r.Reconcile(ctx, mdmsync.SyncModePartial)
		require.NoError(t, err)
		// Alice and converted Bob user equals to 2 imported users.
		require.Equal(t, 2, result.ImportedUsers)

		users, err = listTeleportUsers(ctx, env.cfg.AccessPoint, env.cfg.SSOConnectorID)
		require.NoError(t, err)

		// Counts for Alice and Bob.
		require.Len(t, users, 2)
		require.Equal(t, types.OriginEntraID, users["bob@example.com"].Origin(), "Bob expected to be overwritten")

		env.fakeGraphServer.DeleteUsers([]string{msgraphtest.BobID})
		result, err = r.Reconcile(ctx, mdmsync.SyncModePartial)
		require.NoError(t, err)

		// Only Alice remains and Bob was deleted.
		require.Equal(t, 1, result.ImportedUsers)
		users, err = listTeleportUsers(ctx, env.cfg.AccessPoint, env.cfg.SSOConnectorID)
		require.NoError(t, err)
		require.Len(t, users, 1)
	})
}

func TestUserResourceEqualsInDeltaAndFullSync_WithoutGroupMembershp(t *testing.T) {
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

	cmpIgnore := cmpopts.IgnoreFields(types.Metadata{}, "Revision")

	// Setup test env with msgraphtest fake server.
	graphStorage := msgraphtest.NewStorage()
	graphStorage.Applications = msgraphtest.NewDefaultStorage().Applications
	// Create one Entra ID user Bob.
	graphStorage.Users = make(map[string]*models.User)
	graphStorage.Users[msgraphtest.BobID] = bob

	env := newFakeEnv(t, withFakeEnvStorage(graphStorage))
	env.cfg.DeltaSyncEnabled = true
	r, err := New(env.cfg)
	require.NoError(t, err)

	// Initial full sync
	_, err = r.Reconcile(ctx, mdmsync.SyncModeFull)
	require.NoError(t, err)

	bobResourceAfterFullSync, err := env.identitySvc.GetUser(ctx, bobUsername, false)
	require.NoError(t, err, "expected Bob account in Teleport backend")

	// Sanity check basic user attributes.
	expectBasicUserAttributes(t, bob, bobResourceAfterFullSync)

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
}

func TestUserResourceEqualsInDeltaAndFullSync_WithGroupMembership(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Default user.
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
	// Setup test env with msgraphtest fake server.
	defaultStorage := msgraphtest.NewDefaultStorage()
	defaultGroups := map[string]*models.Group{
		msgraphtest.Group1ID: defaultStorage.Groups[msgraphtest.Group1ID],
		msgraphtest.Group2ID: defaultStorage.Groups[msgraphtest.Group2ID],
	}

	cmpIgnore := cmpopts.IgnoreFields(types.Metadata{}, "Revision")

	testCases := []struct {
		name string
		// applicationProperties to be added to the enterprise application.
		applicationProperties []string
		// groups defines incoming entra group with custom properties such as OnPremisesNetBiosName, OnPremisesSamAccountName etc.
		groups              map[string]*models.Group
		roleMapping         []types.AttributeMapping
		expectedTraitName   string
		expectedTraitValues []string
		expectedRoles       []string
	}{
		{
			name:                  "defaults to groups claim with group id",
			applicationProperties: nil,
			roleMapping: []types.AttributeMapping{
				{Name: entraIDSAMLClaimGroups, Value: msgraphtest.Group1ID, Roles: []string{"access"}},
				{Name: entraIDSAMLClaimGroups, Value: msgraphtest.Group2ID, Roles: []string{"editor"}},
			},
			groups:              defaultGroups,
			expectedTraitName:   entraIDSAMLClaimGroups,
			expectedTraitValues: []string{msgraphtest.Group1ID, msgraphtest.Group2ID},
			expectedRoles:       []string{"access", "editor"},
		},
		{
			name: "default with emit as roles",
			applicationProperties: []string{
				models.OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_EMIT_AS_ROLES,
			},
			groups: defaultGroups,
			roleMapping: []types.AttributeMapping{
				{Name: entraIDSAMLClaimRoles, Value: msgraphtest.Group1ID, Roles: []string{"access"}},
				{Name: entraIDSAMLClaimRoles, Value: msgraphtest.Group2ID, Roles: []string{"editor"}},
			},
			expectedTraitName:   entraIDSAMLClaimRoles,
			expectedTraitValues: []string{msgraphtest.Group1ID, msgraphtest.Group2ID},
			expectedRoles:       []string{"access", "editor"},
		},
		{
			name:                  "default with security and office365 group",
			applicationProperties: []string{},
			groups: map[string]*models.Group{
				msgraphtest.Group1ID: func() *models.Group {
					g := newEntraGroup(t, msgraphtest.Group1ID, "group1")
					g.GroupTypes = []string{"Unified"} // Office 365 group
					return g
				}(),
				msgraphtest.Group2ID: func() *models.Group {
					g := newEntraGroup(t, msgraphtest.Group2ID, "group2") // non-Office 365 group
					return g
				}(),
			},
			roleMapping: []types.AttributeMapping{
				{Name: entraIDSAMLClaimGroups, Value: msgraphtest.Group1ID, Roles: []string{"access"}},
				{Name: entraIDSAMLClaimGroups, Value: msgraphtest.Group2ID, Roles: []string{"editor"}},
			},
			expectedTraitName:   entraIDSAMLClaimGroups,
			expectedTraitValues: []string{msgraphtest.Group2ID}, // group1 filtered out from traits
			expectedRoles:       []string{"editor"},             // access role does not match
		},
		{
			name:                  "sam account name",
			applicationProperties: []string{models.OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_SAM_ACCOUNT_NAME},
			groups: map[string]*models.Group{
				msgraphtest.Group1ID: func() *models.Group {
					g := newEntraGroup(t, msgraphtest.Group1ID, "group1")
					g.OnPremisesSamAccountName = to.Ptr("group1-sam")
					return g
				}(),
				msgraphtest.Group2ID: func() *models.Group {
					g := newEntraGroup(t, msgraphtest.Group2ID, "group2")
					g.OnPremisesSamAccountName = to.Ptr("group2-sam")
					return g
				}(),
			},
			roleMapping: []types.AttributeMapping{
				{Name: entraIDSAMLClaimGroups, Value: "group1-sam", Roles: []string{"access"}},
				{Name: entraIDSAMLClaimGroups, Value: "group2-sam", Roles: []string{"editor"}},
			},
			expectedTraitName:   entraIDSAMLClaimGroups,
			expectedTraitValues: []string{"group1-sam", "group2-sam"},
			expectedRoles:       []string{"access", "editor"},
		},
		{
			name:                  "sam account name fallback to group id",
			applicationProperties: []string{models.OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_SAM_ACCOUNT_NAME},
			groups:                defaultGroups,
			roleMapping: []types.AttributeMapping{
				{Name: entraIDSAMLClaimGroups, Value: msgraphtest.Group1ID, Roles: []string{"access"}},
				{Name: entraIDSAMLClaimGroups, Value: msgraphtest.Group2ID, Roles: []string{"editor"}},
			},
			expectedTraitName:   entraIDSAMLClaimGroups,
			expectedTraitValues: []string{msgraphtest.Group1ID, msgraphtest.Group2ID},
			expectedRoles:       []string{"access", "editor"},
		},
		{
			name: "emit sam account names as roles",
			applicationProperties: []string{
				models.OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_SAM_ACCOUNT_NAME,
				models.OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_EMIT_AS_ROLES,
			},
			groups: map[string]*models.Group{
				msgraphtest.Group1ID: func() *models.Group {
					g := newEntraGroup(t, msgraphtest.Group1ID, "group1")
					g.OnPremisesSamAccountName = to.Ptr("group1-sam")
					return g
				}(),
				msgraphtest.Group2ID: func() *models.Group {
					g := newEntraGroup(t, msgraphtest.Group2ID, "group2")
					g.OnPremisesSamAccountName = to.Ptr("group2-sam")
					return g
				}(),
			},
			roleMapping: []types.AttributeMapping{
				{Name: entraIDSAMLClaimRoles, Value: "group1-sam", Roles: []string{"access"}},
				{Name: entraIDSAMLClaimRoles, Value: "group2-sam", Roles: []string{"editor"}},
			},
			expectedTraitName:   entraIDSAMLClaimRoles,
			expectedTraitValues: []string{"group1-sam", "group2-sam"},
			expectedRoles:       []string{"access", "editor"},
		},
		{
			name:                  "dns domain and sam account name",
			applicationProperties: []string{models.OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_DNS_DOMAIN_AND_SAM_ACCOUNT_NAME},
			groups: map[string]*models.Group{
				msgraphtest.Group1ID: func() *models.Group {
					g := newEntraGroup(t, msgraphtest.Group1ID, "group1")
					g.OnPremisesDomainName = to.Ptr("group1.example.com")
					g.OnPremisesSamAccountName = to.Ptr("group1-sam")
					return g
				}(),
				msgraphtest.Group2ID: func() *models.Group {
					g := newEntraGroup(t, msgraphtest.Group2ID, "group2")
					g.OnPremisesDomainName = to.Ptr("group2.example.com")
					g.OnPremisesSamAccountName = to.Ptr("group2-sam")
					return g
				}(),
			},
			roleMapping: []types.AttributeMapping{
				{Name: entraIDSAMLClaimGroups, Value: `group1.example.com\group1-sam`, Roles: []string{"access"}},
				{Name: entraIDSAMLClaimGroups, Value: `group2.example.com\group2-sam`, Roles: []string{"editor"}},
			},
			expectedTraitName:   entraIDSAMLClaimGroups,
			expectedTraitValues: []string{`group1.example.com\group1-sam`, `group2.example.com\group2-sam`},
			expectedRoles:       []string{"access", "editor"},
		},
		{
			name:                  "netbios domain and sam account name",
			applicationProperties: []string{models.OPTIONAL_CLAIM_ADDITIONAL_PROPERTIES_NETBIOS_DOMAIN_AND_SAM_ACCOUNT_NAME},
			groups: map[string]*models.Group{
				msgraphtest.Group1ID: func() *models.Group {
					g := newEntraGroup(t, msgraphtest.Group1ID, "group1")
					g.OnPremisesNetBiosName = to.Ptr("group1-nb")
					g.OnPremisesSamAccountName = to.Ptr("group1-sam")
					return g
				}(),
				msgraphtest.Group2ID: func() *models.Group {
					g := newEntraGroup(t, msgraphtest.Group2ID, "group2")
					g.OnPremisesNetBiosName = to.Ptr("group2-nb")
					g.OnPremisesSamAccountName = to.Ptr("group2-sam")
					return g
				}(),
			},
			roleMapping: []types.AttributeMapping{
				{Name: entraIDSAMLClaimGroups, Value: `group1-nb\group1-sam`, Roles: []string{"access"}},
				{Name: entraIDSAMLClaimGroups, Value: `group2-nb\group2-sam`, Roles: []string{"editor"}},
			},
			expectedTraitName:   entraIDSAMLClaimGroups,
			expectedTraitValues: []string{`group1-nb\group1-sam`, `group2-nb\group2-sam`},
			expectedRoles:       []string{"access", "editor"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup test env with msgraphtest fake server.
			graphStorage := msgraphtest.NewStorage()
			graphStorage.Applications = defaultStorage.Applications
			graphStorage.Applications[msgraphtest.App1ID].OptionalClaims = &models.OptionalClaims{
				SAML2Token: []models.OptionalClaim{
					{
						Name:                 to.Ptr(models.OPTIONAL_CLAIM_GROUP_NAME),
						AdditionalProperties: tc.applicationProperties,
					},
				},
			}
			// Create one Entra ID user Bob.
			graphStorage.Users = make(map[string]*models.User)
			graphStorage.Users[msgraphtest.BobID] = bob
			graphStorage.Groups = tc.groups
			graphStorage.GroupMembers = make(map[string][]models.GroupMember)
			graphStorage.GroupMembers[msgraphtest.Group1ID] = []models.GroupMember{bob}
			graphStorage.GroupMembers[msgraphtest.Group2ID] = []models.GroupMember{bob}

			connector := mustNewSAMLConnectorWithRoleMapping(t, ssoConnectorID, tc.roleMapping)
			env := newFakeEnv(t, withFakeEnvStorage(graphStorage), withFakeEnvConnector(connector))
			env.cfg.DeltaSyncEnabled = true
			r, err := New(env.cfg)
			require.NoError(t, err)

			// Initial full sync
			_, err = r.Reconcile(ctx, mdmsync.SyncModeFull)
			require.NoError(t, err)

			bobResourceAfterFullSync, err := env.identitySvc.GetUser(ctx, bobUsername, false)
			require.NoError(t, err)

			// Sanity check basic user attributes.
			expectBasicUserAttributes(t, bob, bobResourceAfterFullSync)

			// Check role and traits.
			// Bob is member of Group1ID and Group2ID.
			require.ElementsMatch(t, tc.expectedTraitValues, bobResourceAfterFullSync.GetTraits()[tc.expectedTraitName], "group traits")
			require.ElementsMatch(t, tc.expectedRoles, bobResourceAfterFullSync.GetRoles(), "roles differ")

			// Run reconciler mimicking delta sync.
			_, err = r.Reconcile(ctx, mdmsync.SyncModePartial)
			require.NoError(t, err)
			bobResourceAfterDeltaSync, err := env.identitySvc.GetUser(ctx, bobUsername, false)
			require.NoError(t, err, "expected Bob account to be found after delta sync")

			require.Empty(t, cmp.Diff(bobResourceAfterFullSync, bobResourceAfterDeltaSync, cmpIgnore))
		})
	}
}

func expectBasicUserAttributes(t *testing.T, expected *models.User, got types.User) {
	t.Helper()

	// Check traits matches.
	// First item expected to match from traits values.
	traits := got.GetTraits()
	require.Equal(t, *expected.UserPrincipalName, traits[entraIDSAMLClaimName][0], "username")
	require.Equal(t, tenantID, traits[tenantIDClaim][0], "tenant ID")
	require.Equal(t, *expected.GetID(), traits[objectIdentifierClaim][0], "user object ID")
	require.Equal(t, *expected.DisplayName, traits[displayNameClaim][0], "display name")
	require.Equal(t, *expected.GivenName, traits[entraIDSAMLGivenName][0], "given name")
	require.Equal(t, *expected.Surname, traits[entraIDSAMLSurname][0], "surname")

	// Check labels matches.
	require.Equal(t, types.OriginEntraID, got.Origin(), "user origin")
	labels := got.GetAllLabels()
	require.Equal(t, *expected.GetID(), labels[types.EntraUniqueIDLabel], "Entra unique ID")
	require.Equal(t, tenantID, labels[types.EntraTenantIDLabel], "Entra tenant ID")
	require.Equal(t, *expected.UserPrincipalName, labels[types.EntraUPNLabel], "Entra UPN")
	require.Equal(t, *expected.OnPremisesSAMAccountName, labels[types.EntraSAMAccountNameLabel], "Entra SAM account name")

	isExternal, err := strconv.ParseBool(labels[types.TeleportInternalLabelPrefix+"entra-is-external"])
	require.NoError(t, err)
	require.False(t, isExternal, "entra-is-external label")

	// Check CreatedBy matches.
	require.Equal(t, constants.SAML, got.GetCreatedBy().Connector.Type, "user CreatedBy connector type")
	require.Equal(t, ssoConnectorID, got.GetCreatedBy().Connector.ID, "user CreatedBy connector ID")
	require.Equal(t, *expected.UserPrincipalName, got.GetCreatedBy().Connector.Identity, "user CreatedBy identity")
}

func mustNewSAMLConnectorWithRoleMapping(t *testing.T, connectorID string, roleMapping []types.AttributeMapping) types.SAMLConnector {
	t.Helper()

	connector, err := types.NewSAMLConnector(
		connectorID,
		types.SAMLConnectorSpecV2{
			AssertionConsumerService: "http://localhost:65535/acs", // not called
			Issuer:                   "test",
			SSO:                      "https://localhost:65535/sso", // not called
			AttributesToRoles:        roleMapping,
		})
	require.NoError(t, err)
	return connector
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
	g1ACLName := al1.GetName()
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

	// Of the 4 test groups we started with, mock that group g1 display is
	// updated, g2 is deleted, but two new groups g5 and g6 are added in Entra ID.
	g1.DisplayName = to.Ptr("g1-renamed")
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

	// g1 display name updated but the ACL name remains the same and should be matched.
	require.Equal(t, g1ACLName, al1.GetName())
	require.Equal(t, "g1-renamed", al1.Spec.Title)

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

	// Let's create a cycle, by g4, going back to g1.
	// Expect one of the membership to be filtered to
	// break cyclic relationship.
	graphClient.groupMembers = map[string][]models.GroupMember{
		"g1": {g2},
		"g2": {g3},
		"g3": {g4},
		"g4": {g1},
	}

	_ /* result */, err = r.Reconcile(ctx, mdmsync.SyncModeFull)
	require.NoError(t, err, "cyclic membership should not cause error")
	// One of the membership is filtered due to cyclic membership.
	requireMembersCount(t, env.aclSvc, 3)
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

	env := newFakeEnv(t, withFakeEnvStorage(graphStorage))
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

type fakeEnvConfig struct {
	storage   *msgraphtest.Storage
	connector types.SAMLConnector
}

type fakeEnvOpt func(*fakeEnvConfig)

func withFakeEnvStorage(storage *msgraphtest.Storage) fakeEnvOpt {
	return func(cfg *fakeEnvConfig) {
		cfg.storage = storage
	}
}

func withFakeEnvConnector(connector types.SAMLConnector) fakeEnvOpt {
	return func(cfg *fakeEnvConfig) {
		cfg.connector = connector
	}
}

// newFakeEnv creates new [directoryReconcilerEnv] based on msgraphtest fake server.
func newFakeEnv(t *testing.T, opts ...fakeEnvOpt) directoryReconcilerEnv {
	t.Helper()

	cfg := fakeEnvConfig{
		storage: msgraphtest.NewDefaultStorage(),
	}
	for _, opt := range opts {
		opt(&cfg)
	}

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

	if cfg.connector == nil {
		cfg.connector = newSAMLConnector(t, ssoConnectorID, msgraphtest.Group1ID, msgraphtest.Group2ID)
	}
	_, err = samlService.CreateSAMLConnector(t.Context(), cfg.connector)
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
	if cfg.storage == nil {
		cfg.storage = msgraphtest.NewDefaultStorage()
	}
	fakeServer := msgraphtest.NewServer(msgraphtest.WithStorage(cfg.storage))
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

	reconcilerConfig := Config{
		Clock:       clockwork.NewRealClock(),
		Logger:      logtest.NewLogger(),
		GraphClient: graphClient,
		AccessPoint: ap{
			accessListAccessPoint: alSvc,
			userAccessPoint:       identitySvc,
			connectorAccessPoint:  samlService,
		},
		DefaultOwners:  defaultOwners,
		SSOConnectorID: cfg.connector.GetName(),
		TenantID:       tenantID,
		EntraAppID:     msgraphtest.App1ID,
	}
	require.NoError(t, reconcilerConfig.Validate())

	return directoryReconcilerEnv{
		cfg:             reconcilerConfig,
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

func TestUserReconcilerGoroutineLimit(t *testing.T) {
	const minUsersCountThreshold = 500
	const maxUserCount = 5_000 // large enough to trigger concurrent reconciliation unless overridden.

	// Truthy and falsy values accepted by [libutils.AsBool].
	truthyEnvValues := rapid.SampledFrom(([]string{"yes", "yeah", "y", "true", "1", "on"}))
	// Falsy values explicitly accepted by [libutils.AsBool].
	falsyEnvValues := rapid.SampledFrom([]string{"no", "nope", "n", "false", "0", "off"})
	nonTruthyEnvValues := rapid.OneOf(
		rapid.Just(""),
		falsyEnvValues,
		rapid.Map(
			rapid.String(),
			func(value string) string {
				return "random:" + value // Prefix random string to avoid generating truthy value.
			}),
	)

	t.Run("truthy env var returns minReconcilerGoroutineLimit", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			// Override reconciler to run in a sequential mode.
			envValue := truthyEnvValues.Draw(t, "env_value")
			// Users count large enough to trigger concurrent reconciler.
			entraUsers := rapid.IntRange(minUsersCountThreshold, maxUserCount).Draw(t, "entra_users")
			teleportUsers := rapid.IntRange(minUsersCountThreshold, maxUserCount).Draw(t, "teleport_users")

			got := userReconcilerGoroutineLimit(
				entraUsers,
				teleportUsers,
				libutils.AsBool(envValue),
			)
			// Expect [minReconcilerGoroutineLimit] despite having larger users count.
			require.Equal(t, minReconcilerGoroutineLimit, got,
				"got env=%q entra_users=%d teleport_users=%d", envValue, entraUsers, teleportUsers,
			)
		})
	})

	t.Run("smaller users count returns minReconcilerGoroutineLimit without override", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			envValue := nonTruthyEnvValues.Draw(t, "env_value")
			entraUsers := rapid.IntRange(0, minUsersCountThreshold-1).Draw(t, "entra_users")
			teleportUsers := rapid.IntRange(0, minUsersCountThreshold-1).Draw(t, "teleport_users")

			got := userReconcilerGoroutineLimit(
				entraUsers,
				teleportUsers,
				libutils.AsBool(envValue),
			)
			// Expect [minReconcilerGoroutineLimit] despite having false env var
			// because users count are small enough to warrant sequential reconciliation.
			require.Equal(t, minReconcilerGoroutineLimit, got,
				"got env=%q entra_users=%d teleport_users=%d", envValue, entraUsers, teleportUsers,
			)
		})
	})

	t.Run("larger users count returns maxReconcilerGoroutineLimit without override", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			envValue := nonTruthyEnvValues.Draw(t, "env_value")

			// Create baseline that ranges from zero to max user count.
			entraUsers := rapid.IntRange(0, maxUserCount).Draw(t, "entra_users")
			teleportUsers := rapid.IntRange(0, maxUserCount).Draw(t, "teleport_users")

			// Ensure either entra user or teleport user exceeds the max threshold
			// so that [userReconcilerGoroutineLimit] returns max goroutine value.
			if rapid.Bool().Draw(t, "large_user_set") {
				entraUsers = rapid.IntRange(
					minUsersCountThreshold,
					maxUserCount,
				).Draw(t, "large_entra_users")
			} else {
				teleportUsers = rapid.IntRange(
					minUsersCountThreshold,
					maxUserCount,
				).Draw(t, "large_teleport_users")
			}

			got := userReconcilerGoroutineLimit(
				entraUsers,
				teleportUsers,
				libutils.AsBool(envValue),
			)
			// Expect [maxReconcilerGoroutineLimit] because the env var provides
			// falsy value and the users count is larger than 500.
			require.Equal(t, maxReconcilerGoroutineLimit, got,
				"got env=%q entra_users=%d teleport_users=%d", envValue, entraUsers, teleportUsers,
			)
		})
	})
}

// TestDirectoryReconciler_AccessListName exercises the following
// test cases:
//   - Access List names are unique and cannot collide.
//   - Updating group display names have no effect on Access List resoruce names.
//   - Rolling out the patch with genAccessListName does not churn resource names
//     of existing Entra Access Lists.
//
// Subtests share the same state and is not expected to run independently.
func TestDirectoryReconciler_AccessListName(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	const (
		// legacyGroup1 and legacyGroup2 mimics existing Entra ID Access Lists
		// that are created by using now deleted Access List name function.
		// The older method used immutable group ID with mutable display name
		// value, and joined the values using path.Join. That caused unnecessary
		// resource name change when display name is updated and was also
		// vulnerable to create collision prone names.
		// The test below should prove that this existing resource name must be
		// preserved in a subsequent full or delta sync, even when display name
		// is updated.
		legacyGroup1ID      = "ac43ee82-ecfc-4c4d-ad62-c69f1e7b888f"
		legacyGroup1Display = "legacyGroup1Display"
		legacyGroup2ID      = "37829c52-ff61-4a13-b159-0a4dbbe65a72"
		legacyGroup2Display = "legacyGroup2Display"
	)

	legacyGroup1ACLName := deprecatedAccessListName(legacyGroup1Display, legacyGroup1ID)
	legacyGroup2ACLName := deprecatedAccessListName(legacyGroup2Display, legacyGroup2ID)
	// simulate legacy name collision with spoofed display name, legacyGroup2 -> legacyGroup1
	spoofedLegacy2Display := fmt.Sprintf("../%s/%s", legacyGroup1ID, legacyGroup1Display)
	require.Equal(t, legacyGroup1ACLName, deprecatedAccessListName(spoofedLegacy2Display, legacyGroup2ID))

	// group1 mimics group that is created using the new accessListName function
	// that uses immutable tenant ID and group ID as the seed of the UUID
	// function, which is more stable to create collision free names.
	const (
		group1ID          = "2ff863b1-3b12-4118-a98d-8ec135fb0496"
		group1DisplayName = "group1DisplayName"
	)
	group1ACLName := genAccessListName(tenantID, group1ID).String()

	for _, mode := range []mdmsync.SyncMode{mdmsync.SyncModeFull, mdmsync.SyncModePartial} {
		t.Run(FriendlySyncMode(mode), func(t *testing.T) {
			// Shared fixtures defined in msgraphtest server.
			defaultFakeUsers := msgraphtest.NewDefaultStorage().Users
			alice := defaultFakeUsers[msgraphtest.AliceID]
			bob := defaultFakeUsers[msgraphtest.BobID]
			carol := defaultFakeUsers[msgraphtest.CarolID]

			// Entra groups.
			legacyGroup1 := msgraphtest.NewEntraGroup(legacyGroup1ID, legacyGroup1Display)
			legacyGroup2 := msgraphtest.NewEntraGroup(legacyGroup2ID, legacyGroup2Display)
			group1 := msgraphtest.NewEntraGroup(group1ID, group1DisplayName)

			// Build fake Graph API storage.
			storage := msgraphtest.NewStorage()
			storage.Applications = msgraphtest.NewDefaultStorage().Applications
			storage.Users = defaultFakeUsers

			storage.Groups[legacyGroup1ID] = legacyGroup1
			storage.GroupMembers[legacyGroup1ID] = []models.GroupMember{alice}

			storage.Groups[legacyGroup2ID] = legacyGroup2
			storage.GroupMembers[legacyGroup2ID] = []models.GroupMember{carol}

			storage.Groups[group1ID] = group1
			storage.GroupMembers[group1ID] = []models.GroupMember{bob}

			// Test env.
			env := newFakeEnv(t, withFakeEnvStorage(storage))
			if mode == mdmsync.SyncModePartial {
				env.cfg.DeltaSyncEnabled = true
			}

			expectACL := func(t *testing.T, group *models.Group, groupID, groupDisplay, aclName string, member *models.User) {
				t.Helper()

				accessList := requireAccessListForEntraGroupExists(t, env.aclSvc, group)
				require.Equal(t, aclName, accessList.GetName())
				require.Equal(t, groupID, accessList.GetAllLabels()[types.EntraUniqueIDLabel])
				require.Equal(t, groupDisplay, accessList.Spec.Title)
				require.Equal(t, []string{groupID}, accessList.Spec.Grants.Traits[eteleport.EntraMemberOfGroupTrait])

				requireMemberExists(t, env.aclSvc, accessList, *member.Mail)
			}

			// Manually create Access List for legacy group to create its name
			// using older deprecatedAccessListName function.

			nameResolver := aclNameResolver{
				namesByID: aclNamesByGroupID{
					entraUniqueID(legacyGroup1ID): accessListName(legacyGroup1ACLName),
					entraUniqueID(legacyGroup2ID): accessListName(legacyGroup2ACLName),
				},
			}
			_, legacyGroup1ACL, err := convertGroup(legacyGroup1, tenantID, env.cfg.DefaultOwners, nameResolver)
			require.NoError(t, err)
			_, err = env.aclSvc.UpsertAccessList(ctx, legacyGroup1ACL)
			require.NoError(t, err)

			_, legacyGroup2ACL, err := convertGroup(legacyGroup2, tenantID, env.cfg.DefaultOwners, nameResolver)
			require.NoError(t, err)
			_, err = env.aclSvc.UpsertAccessList(ctx, legacyGroup2ACL)
			require.NoError(t, err)

			// New directory reconciler.
			reconciler, err := New(env.cfg)
			require.NoError(t, err)

			// First sync, is always full sync.
			result, err := reconciler.Reconcile(ctx, mdmsync.SyncModeFull)
			require.NoError(t, err)
			require.NoError(t, result.ErrSkippedResources)
			require.Equal(t, 3, result.ImportedGroups)

			// Expect legacyGroup1,legacyGroup2 and group1 ACL to have been reconciled.
			requireAccessListCount(t, env.aclSvc, 3)
			requireMembersCount(t, env.aclSvc, 3)
			expectACL(t, legacyGroup1, legacyGroup1ID, legacyGroup1Display, legacyGroup1ACLName, alice) // legacy name preserved
			expectACL(t, legacyGroup2, legacyGroup2ID, legacyGroup2Display, legacyGroup2ACLName, carol) // legacy name preserved
			expectACL(t, group1, group1ID, group1DisplayName, group1ACLName, bob)

			// Subsequent sync.

			const group1UpdatedDisplay = "group1-display-updated"

			// group1UpdatedDisplay := new(string)

			t.Run("in-cache group with new name, preserves acl name on display update", func(t *testing.T) {
				// group1.DisplayName = new(string)
				*group1.DisplayName = group1UpdatedDisplay
				// *group1.DisplayName = group1UpdatedDisplay
				env.fakeGraphServer.SetGroups([]*models.Group{group1})

				result, err := reconciler.Reconcile(ctx, mode)
				require.NoError(t, err)
				require.NoError(t, result.ErrSkippedResources)
				require.Equal(t, 3, result.ImportedGroups)

				requireAccessListCount(t, env.aclSvc, 3)
				requireMembersCount(t, env.aclSvc, 3)

				// legacyGroup1 and legacyGroup2 should remain unchanged.
				expectACL(t, legacyGroup1, legacyGroup1ID, legacyGroup1Display, legacyGroup1ACLName, alice)
				expectACL(t, legacyGroup2, legacyGroup2ID, legacyGroup2Display, legacyGroup2ACLName, carol)
				expectACL(t, group1, group1ID, group1UpdatedDisplay, group1ACLName, bob)
			})

			// Subsequent sync with updated legacyGroup1's display.
			const legacyGroup1UpdatedDisplayName = "legacy-abc-updated"

			t.Run("in-cache group with deprecatedAccessListName, preserves acl name on display update", func(t *testing.T) {
				legacyGroup1.DisplayName = to.Ptr(legacyGroup1UpdatedDisplayName)
				env.fakeGraphServer.SetGroups([]*models.Group{legacyGroup1})

				result, err := reconciler.Reconcile(ctx, mode)
				require.NoError(t, err)
				require.NoError(t, result.ErrSkippedResources)
				require.Equal(t, 3, result.ImportedGroups)

				requireAccessListCount(t, env.aclSvc, 3)
				requireMembersCount(t, env.aclSvc, 3)

				// legacyGroup1 display updated. legacyGroup2 and group1 remains unchanged.
				expectACL(t, legacyGroup1, legacyGroup1ID, legacyGroup1UpdatedDisplayName, legacyGroup1ACLName, alice)
				expectACL(t, legacyGroup2, legacyGroup2ID, legacyGroup2Display, legacyGroup2ACLName, carol)
				expectACL(t, group1, group1ID, group1UpdatedDisplay, group1ACLName, bob)
			})

			// Check for name collision legacyGroup1 -> legacyGroup2.
			const legacyGroup1SpoofedDisplayName = "../" + legacyGroup2ID + "/" + legacyGroup2Display

			t.Run("in-cache with deprecatedAccessListName, spoofing group with deprecatedAccessListName", func(t *testing.T) {
				legacyGroup1.DisplayName = to.Ptr(legacyGroup1SpoofedDisplayName)
				env.fakeGraphServer.SetGroups([]*models.Group{legacyGroup1})

				result, err := reconciler.Reconcile(ctx, mode)
				require.NoError(t, err)
				require.NoError(t, result.ErrSkippedResources)
				require.Equal(t, 3, result.ImportedGroups)

				requireAccessListCount(t, env.aclSvc, 3)
				requireMembersCount(t, env.aclSvc, 3)

				// Only legacyGroup1 title is updated, acl resource names of both legacyGroup1 and legacyGroup2 remains the same.
				expectACL(t, legacyGroup1, legacyGroup1ID, legacyGroup1SpoofedDisplayName, legacyGroup1ACLName, alice)
				expectACL(t, legacyGroup2, legacyGroup2ID, legacyGroup2Display, legacyGroup2ACLName, carol)
				expectACL(t, group1, group1ID, group1UpdatedDisplay, group1ACLName, bob)
			})

			// Check for name collision group1 -> legacyGroup2
			const group1SpoofedDisplayName = "../" + legacyGroup2ID + "/" + legacyGroup2Display

			t.Run("in-cache with group with new name spoofing legacy name", func(t *testing.T) {
				group1.DisplayName = to.Ptr(group1SpoofedDisplayName)
				env.fakeGraphServer.SetGroups([]*models.Group{group1})

				result, err := reconciler.Reconcile(ctx, mode)
				require.NoError(t, err)
				require.NoError(t, result.ErrSkippedResources)
				require.Equal(t, 3, result.ImportedGroups)

				requireAccessListCount(t, env.aclSvc, 3)
				requireMembersCount(t, env.aclSvc, 3)

				// Only group1 title is updated, acl resource names of both group1 and legacyGroup2 remains the same.
				expectACL(t, legacyGroup1, legacyGroup1ID, legacyGroup1SpoofedDisplayName, legacyGroup1ACLName, alice)
				expectACL(t, legacyGroup2, legacyGroup2ID, legacyGroup2Display, legacyGroup2ACLName, carol)
				expectACL(t, group1, group1ID, group1SpoofedDisplayName, group1ACLName, bob)
			})

			// Check for name collision new fresh group group2 -> legacyGroup2
			const (
				group2ID          = "de6bac54-84bd-4848-b9ba-24e2fe46a0a2"
				group2DisplayName = "../" + legacyGroup2ID + "/" + legacyGroup2Display
			)
			group2ACLName := genAccessListName(tenantID, group2ID).String()
			group2 := msgraphtest.NewEntraGroup(group2ID, group2DisplayName)

			t.Run("new group spoofing group id and display (target legacy naming)", func(t *testing.T) {
				// Sanity check that the payload would otherwise collide.
				require.Equal(t, legacyGroup2ACLName, deprecatedAccessListName(group2DisplayName, group2ID))

				env.fakeGraphServer.SetGroups([]*models.Group{group2})
				env.fakeGraphServer.SetGroupMembers(group2ID, []models.GroupMember{alice})

				result, err := reconciler.Reconcile(ctx, mode)
				require.NoError(t, err)
				require.NoError(t, result.ErrSkippedResources)
				require.Equal(t, 4, result.ImportedGroups)

				requireAccessListCount(t, env.aclSvc, 4)
				requireMembersCount(t, env.aclSvc, 4)

				// group2 should be created, existing Access Lists should remain intact.
				expectACL(t, legacyGroup1, legacyGroup1ID, legacyGroup1SpoofedDisplayName, legacyGroup1ACLName, alice)
				expectACL(t, legacyGroup2, legacyGroup2ID, legacyGroup2Display, legacyGroup2ACLName, carol)
				expectACL(t, group1, group1ID, group1SpoofedDisplayName, group1ACLName, bob)
				expectACL(t, group2, group2ID, group2DisplayName, group2ACLName, alice)
			})

			// Check for name collision new fresh group group3 -> group1ID spoofing new format.
			const (
				group3ID          = "c910a899-0665-4a9b-829e-5a9984263e5a"
				group3DisplayName = "../" + tenantID + "/" + group1ID // v2 seed syntax.
			)

			t.Run("new group spoofing tenant and group id (target newer naming)", func(t *testing.T) {
				// Sanity check that the payload would otherwise collide.
				require.Equal(t, tenantID+"/"+group1ID, path.Join(group3ID, group3DisplayName))

				group3ACLName := genAccessListName(tenantID, group3ID).String()
				group3 := msgraphtest.NewEntraGroup(group3ID, group3DisplayName)
				env.fakeGraphServer.SetGroups([]*models.Group{group3})
				env.fakeGraphServer.SetGroupMembers(group3ID, []models.GroupMember{carol})

				result, err := reconciler.Reconcile(ctx, mode)
				require.NoError(t, err)
				require.NoError(t, result.ErrSkippedResources)
				require.Equal(t, 5, result.ImportedGroups)

				requireAccessListCount(t, env.aclSvc, 5)
				requireMembersCount(t, env.aclSvc, 5)

				// group3 should be created, existing Access Lists should remain intact.
				expectACL(t, legacyGroup1, legacyGroup1ID, legacyGroup1SpoofedDisplayName, legacyGroup1ACLName, alice)
				expectACL(t, legacyGroup2, legacyGroup2ID, legacyGroup2Display, legacyGroup2ACLName, carol)
				expectACL(t, group1, group1ID, group1SpoofedDisplayName, group1ACLName, bob)
				expectACL(t, group2, group2ID, group2DisplayName, group2ACLName, alice)
				expectACL(t, group3, group3ID, group3DisplayName, group3ACLName, carol)
			})
		})
	}
}
