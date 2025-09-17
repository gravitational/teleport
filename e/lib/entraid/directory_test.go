package entraid

import (
	"context"
	"maps"
	"slices"
	"sort"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/msgraph"
	"github.com/gravitational/teleport/lib/services/local"
)

type fakeGraphClient struct {
	users        []*msgraph.User
	groups       []*msgraph.Group
	groupMembers map[string][]msgraph.GroupMember
	applications []*msgraph.Application
}

func newFakeGraphClient() *fakeGraphClient {
	return &fakeGraphClient{
		groupMembers: make(map[string][]msgraph.GroupMember),
	}
}

func (c *fakeGraphClient) IterateGroupMembers(ctx context.Context, groupID string, f func(msgraph.GroupMember) bool, opts ...msgraph.IterateOpt) error {
	for _, m := range c.groupMembers[groupID] {
		if !f(m) {
			return nil
		}
	}
	return nil
}

func (c *fakeGraphClient) IterateGroups(ctx context.Context, f func(*msgraph.Group) bool, opts ...msgraph.IterateOpt) error {
	for _, g := range c.groups {
		if !f(g) {
			return nil
		}
	}
	return nil
}

func (c *fakeGraphClient) IterateUsers(ctx context.Context, f func(*msgraph.User) bool, opts ...msgraph.IterateOpt) error {
	for _, u := range c.users {
		if !f(u) {
			return nil
		}
	}
	return nil
}

func (c *fakeGraphClient) IterateApplications(ctx context.Context, f func(*msgraph.Application) bool, opts ...msgraph.IterateOpt) error {
	panic("not implemented")
}

func (c *fakeGraphClient) GetApplication(ctx context.Context, appID string) (*msgraph.Application, error) {
	for _, app := range c.applications {
		if *app.AppID == appID {
			return app, nil
		}
	}

	return nil, trace.NotFound("application %q not found", appID)
}

func TestDirectoryReconciler(t *testing.T) {
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
	env := newDirectoryReconcilerEnv(t, graphClient, connector)

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
	aliceEntra := &msgraph.User{}
	aliceEntra.ID = &aliceID
	aliceEntra.UserPrincipalName = &aliceUPN
	aliceEntra.DisplayName = to.Ptr("Alice Smith")
	aliceEntra.GivenName = to.Ptr("Alice")
	aliceEntra.Surname = to.Ptr("Smith")
	aliceSAMAccountName := "alice-on-prem"
	aliceEntra.OnPremisesSAMAccountName = &aliceSAMAccountName
	graphClient.users = append(graphClient.users, aliceEntra)

	// Team A does not exist in Teleport, but exists in entra. Alice is a member
	teamAEntra := entraGroup(t, teamAID, "Team A")
	graphClient.groups = append(graphClient.groups, teamAEntra)
	graphClient.groupMembers[teamAID] = []msgraph.GroupMember{aliceEntra}

	// Bob exists in both Entra and Teleport, should stay unchanged
	bobEntra := entraUser(t, bobID, "bob@example.com")
	graphClient.users = append(graphClient.users, bobEntra)

	bobTeleport, err := convertUser(bobEntra, env.cfg.TenantID, env.cfg.SSOConnectorID, userMemberships, false /* emitAsRoles */)
	require.NoError(t, err)
	bobTeleport.SetRoles([]string{"access", "editor"})
	sortTraits(bobTeleport)
	bobTeleport, err = env.cfg.UserSvc.CreateUser(ctx, bobTeleport)
	require.NoError(t, err)

	// Team A contains an unconvertable member.
	// It should be gracefully ignored.
	subgroup := entraGroup(t, subgroupID, "foo")
	graphClient.groups = append(graphClient.groups, subgroup)
	graphClient.groupMembers[teamAID] = append(graphClient.groupMembers[teamAID], subgroup)
	graphClient.groupMembers[subgroupID] = append(graphClient.groupMembers[subgroupID], bobEntra)

	// Michael is a guest user in Entra, should be imported as a local user
	michaelEntra := entraUser(t, michaelID, "michael_someothercompany.io#EXT#@example.com")
	graphClient.users = append(graphClient.users, michaelEntra)

	michaelTeleport, err := convertUser(michaelEntra, env.cfg.TenantID, env.cfg.SSOConnectorID, userMemberships, false /* emitAsRoles */)
	require.NoError(t, err)
	michaelTeleport, err = env.cfg.UserSvc.CreateUser(ctx, michaelTeleport)
	require.NoError(t, err)
	require.Equal(t, "michael@someothercompany.io", michaelTeleport.GetName())

	// Carol exists in both, but was recently unassigned from Team C in Entra
	carolEntra := entraUser(t, carolID, "carol@example.com")
	graphClient.users = append(graphClient.users, carolEntra)

	carolTeleport, err := convertUser(carolEntra, env.cfg.TenantID, env.cfg.SSOConnectorID, userMemberships, false /* emitAsRoles */)
	require.NoError(t, err)
	carolTeleport, err = env.cfg.UserSvc.CreateUser(ctx, carolTeleport)
	require.NoError(t, err)

	// Team C exists in both, but members have changed in Entra (Carol was removed)
	teamCEntra := entraGroup(t, uuid.NewString(), "Team C")
	graphClient.groups = append(graphClient.groups, teamCEntra)

	teamCTeleport, err := convertGroup(teamCEntra, env.cfg.TenantID, env.cfg.DefaultOwners)
	teamCTeleport.Spec.Grants.Roles = []string{"access"}
	require.NoError(t, err)

	carolTeamCTeleportMember, err := convertGroupMember(ctx, carolEntra, teamCTeleport, userMap(carolTeleport), nil)
	require.NoError(t, err)
	_, _, err = env.aclSvc.UpsertAccessListWithMembers(ctx, teamCTeleport, []*accesslist.AccessListMember{carolTeamCTeleportMember})
	require.NoError(t, err)

	// Dave exists in Teleport, but was removed from Entra
	daveEntra := entraUser(t, daveID, "dave@example.com")

	daveTeleport, err := convertUser(daveEntra, env.cfg.TenantID, env.cfg.SSOConnectorID, userMemberships, false /* emitAsRoles */)
	require.NoError(t, err)
	_, err = env.cfg.UserSvc.CreateUser(ctx, daveTeleport)
	require.NoError(t, err)

	// Eve has a local account in Teleport, should not get overwritten by her imported Entra account
	eveEntra := entraUser(t, eveID, "eve@example.com")
	graphClient.users = append(graphClient.users, eveEntra)

	eveTeleport, err := types.NewUser(*eveEntra.UserPrincipalName)
	require.NoError(t, err)
	eveTeleport, err = env.cfg.UserSvc.CreateUser(ctx, eveTeleport)
	require.NoError(t, err)

	// Frank has a local account in Teleport and no equivalent in Entra. Must not get modified or deleted.
	frankUPN := "frank@example.com"
	frankTeleport, err := types.NewUser(frankUPN)
	require.NoError(t, err)
	frankTeleport, err = env.cfg.UserSvc.CreateUser(ctx, frankTeleport)
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

	r, err := NewDirectoryReconciler(env.cfg)
	require.NoError(t, err)

	err = r.Reconcile(ctx)
	require.NoError(t, err)

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

		teamATeleportExpected, err := convertGroup(teamAEntra, env.cfg.TenantID, env.cfg.DefaultOwners)
		require.NoError(t, err)
		teamATeleport, err := env.aclSvc.GetAccessList(ctx, teamATeleportExpected.GetName())
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
	const (
		groupID        = "uuid"
		samAccountName = "foo"
		netBiosName    = "bar"
		domainName     = "baz"
	)

	tests := []struct {
		name            string
		optionalClaims  *msgraph.OptionalClaims
		group           *msgraph.Group
		emitAsRoles     bool
		groupTraitValue string
	}{
		{
			name:           "no optional claims",
			optionalClaims: nil,
			group: &msgraph.Group{
				DirectoryObject: msgraph.DirectoryObject{
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
			optionalClaims: &msgraph.OptionalClaims{},
			group: &msgraph.Group{
				DirectoryObject: msgraph.DirectoryObject{
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
			optionalClaims: &msgraph.OptionalClaims{
				SAML2Token: []msgraph.OptionalClaim{
					{
						Name:                 to.Ptr("groups"),
						AdditionalProperties: []string{"sam_account_name"},
					},
				},
			},
			group: &msgraph.Group{
				DirectoryObject: msgraph.DirectoryObject{
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
			optionalClaims: &msgraph.OptionalClaims{
				SAML2Token: []msgraph.OptionalClaim{
					{
						Name:                 to.Ptr("groups"),
						AdditionalProperties: []string{"sam_account_name"},
					},
				},
			},
			group: &msgraph.Group{
				DirectoryObject: msgraph.DirectoryObject{
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
			optionalClaims: &msgraph.OptionalClaims{
				SAML2Token: []msgraph.OptionalClaim{
					{
						Name:                 to.Ptr("groups"),
						AdditionalProperties: []string{"netbios_domain_and_sam_account_name"},
					},
				},
			},
			group: &msgraph.Group{
				DirectoryObject: msgraph.DirectoryObject{
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
			optionalClaims: &msgraph.OptionalClaims{
				SAML2Token: []msgraph.OptionalClaim{
					{
						Name:                 to.Ptr("groups"),
						AdditionalProperties: []string{"netbios_domain_and_sam_account_name"},
					},
				},
			},
			group: &msgraph.Group{
				DirectoryObject: msgraph.DirectoryObject{
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
			optionalClaims: &msgraph.OptionalClaims{
				SAML2Token: []msgraph.OptionalClaim{
					{
						Name:                 to.Ptr("groups"),
						AdditionalProperties: []string{"netbios_domain_and_sam_account_name"},
					},
				},
			},
			group: &msgraph.Group{
				DirectoryObject: msgraph.DirectoryObject{
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
			optionalClaims: &msgraph.OptionalClaims{
				SAML2Token: []msgraph.OptionalClaim{
					{
						Name:                 to.Ptr("groups"),
						AdditionalProperties: []string{"dns_domain_and_sam_account_name"},
					},
				},
			},
			group: &msgraph.Group{
				DirectoryObject: msgraph.DirectoryObject{
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
			optionalClaims: &msgraph.OptionalClaims{
				SAML2Token: []msgraph.OptionalClaim{
					{
						Name:                 to.Ptr("groups"),
						AdditionalProperties: []string{"dns_domain_and_sam_account_name"},
					},
				},
			},
			group: &msgraph.Group{
				DirectoryObject: msgraph.DirectoryObject{
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
			optionalClaims: &msgraph.OptionalClaims{
				SAML2Token: []msgraph.OptionalClaim{
					{
						Name:                 to.Ptr("groups"),
						AdditionalProperties: []string{"dns_domain_and_sam_account_name"},
					},
				},
			},
			group: &msgraph.Group{
				DirectoryObject: msgraph.DirectoryObject{
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
			optionalClaims: &msgraph.OptionalClaims{
				SAML2Token: []msgraph.OptionalClaim{
					{
						Name:                 to.Ptr("groups"),
						AdditionalProperties: []string{"dns_domain_and_sam_account_name", "emit_as_roles"},
					},
				},
			},
			group: &msgraph.Group{
				DirectoryObject: msgraph.DirectoryObject{
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
			app := &msgraph.Application{
				OptionalClaims: tt.optionalClaims,
			}
			emitAsRoles, f := getGroupNameBuilderFunc(app)
			require.Equal(t, tt.emitAsRoles, emitAsRoles)
			require.Equal(t, tt.groupTraitValue, f(tt.group))
		})
	}
}

func TestUserSync(t *testing.T) {
	ctx := t.Context()
	graphClient := newFakeGraphClient()
	env := newDirectoryReconcilerEnv(t, graphClient, nil /* custom saml connector */)

	t.Run("Create user succeeds", func(t *testing.T) {
		graphClient.users = []*msgraph.User{
			entraUser(t, "u1", "alice@example.com"),
			entraUser(t, "u2", "bob@example.com"),
		}
		env.cfg.GraphClient = graphClient

		r, err := NewDirectoryReconciler(env.cfg)
		require.NoError(t, err)
		require.NoError(t, r.Reconcile(ctx))

		users, err := listTeleportUsers(ctx, env.cfg.UserSvc)
		require.NoError(t, err)

		require.Len(t, users, 2)
	})

	t.Run("User account skipped on sanitization error", func(t *testing.T) {
		graphClient.users = []*msgraph.User{
			entraUser(t, "u1", "al'ice@example.com"),
			entraUser(t, "u2", "bob@example.com"),
			entraUser(t, "u3", "carol@example.com"),
		}

		expectedGroups := []*msgraph.Group{
			entraGroup(t, "g1", "apple"),
			entraGroup(t, "g2", "banana"),
		}
		graphClient.groups = expectedGroups

		graphClient.groupMembers = map[string][]msgraph.GroupMember{
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

		r, err := NewDirectoryReconciler(env.cfg)
		require.NoError(t, err)
		require.NoError(t, r.Reconcile(ctx))

		users, err := listTeleportUsers(ctx, env.cfg.UserSvc)
		require.NoError(t, err)
		require.Len(t, users, 2)

		acls, err := listTeleportAccessLists(ctx, env.cfg.AccessListSvc)
		require.NoError(t, err)
		require.Len(t, acls, 2)

		expected := convertEntraAccessLists(t.Context(), entraGroupsMap(t, expectedGroups), env.cfg.TenantID, env.cfg.DefaultOwners)
		require.Empty(t, cmp.Diff(expected, acls, cmpOpts...), "access list(s) doesn't match")

		aclM, err := listTeleportAccessListMembers(ctx, env.cfg.AccessListSvc, slices.Collect(maps.Values(acls)))
		require.NoError(t, err)
		require.True(t, memberExists(t, aclM, aclIDFromGroupID(t, expected, "g1"), "bob@example.com"))
		require.True(t, memberExists(t, aclM, aclIDFromGroupID(t, expected, "g1"), "carol@example.com"))
		require.True(t, memberExists(t, aclM, aclIDFromGroupID(t, expected, "g2"), "bob@example.com"))
		require.True(t, memberExists(t, aclM, aclIDFromGroupID(t, expected, "g2"), "carol@example.com"))
	})

}

type directoryReconcilerEnv struct {
	cfg         DirectoryReconcilerConfig
	identitySvc *local.IdentityService
	aclSvc      *local.AccessListService
}

func newDirectoryReconcilerEnv(t *testing.T, graphClient *fakeGraphClient, connector types.SAMLConnector) directoryReconcilerEnv {
	modulestest.SetTestModules(t, modulestest.Modules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Identity: {Enabled: true},
			},
		},
	})

	clock := clockwork.NewRealClock()
	mem, err := memory.New(memory.Config{})
	require.NoError(t, err)
	bk := backend.NewSanitizer(mem)
	identitySvc, err := local.NewIdentityService(bk)
	require.NoError(t, err)
	alSvc, err := local.NewAccessListService(bk, clock)
	require.NoError(t, err)

	samlService, err := local.NewIdentityService(bk)
	require.NoError(t, err)
	if connector == nil {
		ssoConnectorID := "my-sso-connector"
		connector = newSAMLConnector(t, ssoConnectorID, uuid.NewString(), uuid.NewString())
	}
	_, err = samlService.CreateSAMLConnector(t.Context(), connector)
	require.NoError(t, err)

	applicationID := uuid.NewString()
	application := &msgraph.Application{
		AppID: to.Ptr(applicationID),
		DirectoryObject: msgraph.DirectoryObject{
			DisplayName: to.Ptr("My Application"),
			ID:          to.Ptr(uuid.NewString()),
		},
		OptionalClaims: &msgraph.OptionalClaims{
			SAML2Token: []msgraph.OptionalClaim{
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

	cfg := DirectoryReconcilerConfig{
		GraphClient:    graphClient,
		UserSvc:        identitySvc,
		AccessListSvc:  alSvc,
		DefaultOwners:  defaultOwners,
		SSOConnectorID: connector.GetName(),
		TenantID:       tenantID,
		SAMLSvc:        samlService,
		EntraAppID:     applicationID,
	}
	require.NoError(t, cfg.Validate())

	return directoryReconcilerEnv{
		cfg:         cfg,
		identitySvc: identitySvc,
		aclSvc:      alSvc,
	}
}

var cmpOpts = []cmp.Option{
	cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
	cmpopts.IgnoreFields(accesslist.Status{}, "OwnerOf"),
	cmpopts.IgnoreFields(accesslist.Status{}, "MemberOf"),
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

func entraGroup(t *testing.T, id, name string) *msgraph.Group {
	t.Helper()
	return &msgraph.Group{
		DirectoryObject: msgraph.DirectoryObject{
			ID:          to.Ptr(id),
			DisplayName: to.Ptr(name),
		},
	}
}

func entraUser(t *testing.T, id, mail string) *msgraph.User {
	t.Helper()
	return &msgraph.User{
		DirectoryObject: msgraph.DirectoryObject{
			ID: to.Ptr(id),
		},
		UserPrincipalName: to.Ptr(mail),
		Mail:              to.Ptr(mail),
	}
}

func entraGroupsMap(t *testing.T, groups []*msgraph.Group) map[string]*msgraph.Group {
	t.Helper()
	result := make(map[string]*msgraph.Group, len(groups))
	for _, g := range groups {
		result[*g.ID] = g
	}
	return result
}

func aclIDFromGroupID(t *testing.T, in map[string]*accesslist.AccessList, gid string) string {
	t.Helper()
	for _, a := range in {
		if id, ok := a.Metadata.GetStaticLabels()[types.EntraUniqueIDLabel]; ok {
			if id == gid {
				return a.GetName()
			}
		}
	}
	return ""
}

func memberExists(t *testing.T, in map[string]*accesslist.AccessListMember, acl, user string) bool {
	t.Helper()
	for _, a := range in {
		if a.Spec.AccessList == acl && a.GetName() == user {
			return true
		}
	}
	return false
}
