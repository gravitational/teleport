package directory

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/msgraph/models"
	"github.com/gravitational/teleport/lib/plugins/filter"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestProcessUserDelta(t *testing.T) {
	t.Parallel()

	aliceUser, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	aliceLabels := map[string]string{
		types.EntraUniqueIDLabel:                                alice,
		types.EntraTenantIDLabel:                                tenantID,
		types.EntraUPNLabel:                                     "alice@example.com",
		onPremisesSamAccountNameLabel:                           "alice sam name",
		types.TeleportInternalLabelPrefix + "entra-is-external": strconv.FormatBool(false),
	}
	aliceUser.SetStaticLabels(aliceLabels)
	aliceUser.SetOrigin(types.OriginEntraID)
	aliceTraits := map[string][]string{
		entraIDSAMLClaimName:  {"alice@example.com"},
		tenantIDClaim:         {tenantID},
		objectIdentifierClaim: {alice},
		displayNameClaim:      {"Alice A"},
		entraIDSAMLGivenName:  {"Alice"},
		entraIDSAMLSurname:    {"A"},
		entraIDSAMLClaimEmail: {"alice@example.com"},
	}
	aliceUser.SetTraits(aliceTraits)

	entraGroupMembershipmap := groupMembershipMap{
		alice: groupMembershipInfo{
			groupIds:   []string{group1, group2},
			groupNames: []string{"group1", "group2"},
		},
		bob: groupMembershipInfo{
			groupIds:   []string{group1},
			groupNames: []string{"group1"},
		},
	}

	testCases := []struct {
		name               string
		userDelta          *models.ListUsersDeltaResponse
		expectedUsername   string
		expectedLabels     map[string]string
		expectedTraits     map[string][]string
		expectedUsersCount int
		isRemoved          bool
	}{
		{
			name: "User added",
			userDelta: &models.ListUsersDeltaResponse{
				User: &models.User{
					DirectoryObject: models.DirectoryObject{
						ID:          to.Ptr("68f6178e-f8a2-4d9f-9ba5-2296f02465b0"),
						DisplayName: to.Ptr("David D"),
					},
					Mail:              to.Ptr("david@example.com"),
					UserPrincipalName: to.Ptr("david@example.com"),
					GivenName:         to.Ptr("David"),
					Surname:           to.Ptr("D"),
				},
			},
			expectedUsername: "david@example.com",
			expectedLabels: map[string]string{
				types.OriginLabel:        types.OriginEntraID,
				types.EntraUniqueIDLabel: "68f6178e-f8a2-4d9f-9ba5-2296f02465b0",
				types.EntraTenantIDLabel: tenantID,
				types.EntraUPNLabel:      "david@example.com",
				types.TeleportInternalLabelPrefix + "entra-is-external": strconv.FormatBool(false),
			},
			expectedTraits: map[string][]string{
				entraIDSAMLClaimName:  {"david@example.com"},
				tenantIDClaim:         {tenantID},
				objectIdentifierClaim: {"68f6178e-f8a2-4d9f-9ba5-2296f02465b0"},
				displayNameClaim:      {"David D"},
				entraIDSAMLGivenName:  {"David"},
				entraIDSAMLSurname:    {"D"},
				entraIDSAMLClaimEmail: {"david@example.com"},
			},
			expectedUsersCount: 4,
		},
		{
			name: "User updated",
			userDelta: &models.ListUsersDeltaResponse{
				User: &models.User{
					DirectoryObject: models.DirectoryObject{
						ID:          to.Ptr(alice),
						DisplayName: to.Ptr("Alice Chains"),
					},
					Mail:                     to.Ptr("alice@example.com"),
					OnPremisesSAMAccountName: to.Ptr("alice new sam name"),
					UserPrincipalName:        to.Ptr("alice@example.com"),
					GivenName:                to.Ptr("Alice C"),
					Surname:                  nil, // removed property is sent as "null"
				},
			},
			expectedUsername: "alice@example.com",
			expectedLabels: map[string]string{
				types.OriginLabel:                                       types.OriginEntraID,
				types.EntraUniqueIDLabel:                                alice,
				types.EntraTenantIDLabel:                                tenantID,
				types.EntraUPNLabel:                                     "alice@example.com",
				types.EntraSAMAccountNameLabel:                          "alice new sam name",
				types.TeleportInternalLabelPrefix + "entra-is-external": strconv.FormatBool(false),
			},
			expectedTraits: map[string][]string{
				entraIDSAMLClaimName:  {"alice@example.com"},
				tenantIDClaim:         {tenantID},
				objectIdentifierClaim: {alice},
				displayNameClaim:      {"Alice Chains"},
				entraIDSAMLGivenName:  {"Alice C"},
				// surname removed
				entraIDSAMLClaimEmail:  {"alice@example.com"},
				entraIDSAMLClaimGroups: {"group1", "group2"},
			},
			expectedUsersCount: 3,
		},
		{
			name: "User unchanged", // could be an update replay
			userDelta: &models.ListUsersDeltaResponse{
				User: &models.User{
					DirectoryObject: models.DirectoryObject{
						ID:          to.Ptr(bob),
						DisplayName: to.Ptr("Bob B"),
					},
					Mail:              to.Ptr("bob@example.com"),
					UserPrincipalName: to.Ptr("bob@example.com"),
				},
			},
			expectedUsername: "bob@example.com",
			expectedLabels: map[string]string{
				types.OriginLabel:        types.OriginEntraID,
				types.EntraUniqueIDLabel: bob,
				types.EntraTenantIDLabel: tenantID,
				types.EntraUPNLabel:      "bob@example.com",
				types.TeleportInternalLabelPrefix + "entra-is-external": strconv.FormatBool(false),
			},
			expectedTraits: map[string][]string{
				entraIDSAMLClaimName:   {"bob@example.com"},
				tenantIDClaim:          {tenantID},
				objectIdentifierClaim:  {bob},
				displayNameClaim:       {"Bob B"},
				entraIDSAMLClaimEmail:  {"bob@example.com"},
				entraIDSAMLClaimGroups: {"group1"},
			},
			expectedUsersCount: 3,
		},
		{
			name: "User removed",
			userDelta: &models.ListUsersDeltaResponse{
				User: &models.User{
					DirectoryObject: models.DirectoryObject{
						ID: to.Ptr(alice),
					},
				},
				Removed: &models.RemovedReason{
					Reason: to.Ptr("deleted"),
				},
			},
			expectedUsersCount: 2,
			isRemoved:          true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare Teleport user map based on Entra ID users.
			teleportUsersMap := map[string]types.User{
				alice: aliceUser,
				bob:   mustTeleportUser("bob@example.com", bob),
				carol: mustTeleportUser("carol@example.com", carol),
			}
			userCfg := userConfig{
				tenantID:       tenantID,
				ssoConnectorID: connectorID,
				emitAsRoles:    false,
			}

			deltaProcessor := newUserDeltaProcessor(
				entraGroupMembershipmap,
				teleportUsersMap,
				userCfg,
			)
			err := deltaProcessor.apply(tc.userDelta)
			require.NoError(t, err)

			result := deltaProcessor.result()
			require.Len(t, result, tc.expectedUsersCount)

			if tc.isRemoved {
				return
			}

			teleportUserOut, ok := result[tc.expectedUsername]
			require.True(t, ok, "expected Teleport user to be found in deltaProcessor newState")

			require.Equal(t, tc.expectedLabels, teleportUserOut.GetAllLabels(), "expected labels to match")
			require.Equal(t, tc.expectedTraits, teleportUserOut.GetTraits(), "expected traits to match")
		})
	}
}

func TestProcessGroupDelta(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	testCases := []struct {
		name                      string
		groupDelta                *models.ListGroupsDeltaResponse
		expectedDisplay           string
		expectedGroupsCount       int
		expectedGroupOwnersCount  int
		expectedGroupMembersCount int
		isRemoved                 bool
	}{
		{
			name: "Group added",
			groupDelta: &models.ListGroupsDeltaResponse{
				Group: &models.Group{
					DirectoryObject: models.DirectoryObject{
						ID:          to.Ptr("7dab05a9-3d2d-4ab1-9e6b-73cea247b7ac"),
						DisplayName: to.Ptr("group4"),
					},
				},
				Owners: []models.OwnersDelta{
					{
						User: &models.User{
							DirectoryObject: models.DirectoryObject{
								ID:          to.Ptr("93c1c88d-fb6e-4bb8-93e9-0d53c62e42c7"),
								DisplayName: to.Ptr("new owner"),
							},
						},
						Type: models.ODataUser,
					},
				},
				Members: []models.MembersDelta{
					{
						DirectoryObject: &models.DirectoryObject{
							ID:          to.Ptr("b8dfb206-7963-43ce-93e3-596141befd56"),
							DisplayName: to.Ptr("new member"),
						},
						Type: models.ODataGroup,
					},
				},
			},
			expectedDisplay:           "group4",
			expectedGroupsCount:       4,
			expectedGroupOwnersCount:  1,
			expectedGroupMembersCount: 1,
		},
		{
			name: "Group display updated",
			groupDelta: &models.ListGroupsDeltaResponse{
				Group: &models.Group{
					DirectoryObject: models.DirectoryObject{
						ID:          to.Ptr(group2),
						DisplayName: to.Ptr("group2 new name"),
					},
				},
			},
			expectedDisplay:           "group2 new name",
			expectedGroupsCount:       3,
			expectedGroupOwnersCount:  2, // expected owners to be the same.
			expectedGroupMembersCount: 2, // expect membership to be the same.
		},
		{
			name: "Group members and owners removed",
			groupDelta: &models.ListGroupsDeltaResponse{
				Group: &models.Group{
					DirectoryObject: models.DirectoryObject{
						ID:          to.Ptr(group1),
						DisplayName: to.Ptr("group1"), // display name is expected when members updated.
					},
				},
				Owners: []models.OwnersDelta{
					{
						User: &models.User{
							DirectoryObject: models.DirectoryObject{
								ID: to.Ptr(alice),
							},
						},
						Type: models.ODataUser,
						Removed: &models.RemovedReason{
							Reason: to.Ptr("changed"),
						},
					},
				},
				Members: []models.MembersDelta{
					{
						DirectoryObject: &models.DirectoryObject{
							ID: to.Ptr(alice),
						},
						Type: models.ODataUser,
						Removed: &models.RemovedReason{
							Reason: to.Ptr("changed"),
						},
					},
					{
						DirectoryObject: &models.DirectoryObject{
							ID: to.Ptr(group2),
						},
						Type: models.ODataGroup,
						Removed: &models.RemovedReason{
							Reason: to.Ptr("changed"),
						},
					},
				},
			},
			expectedDisplay:           "group1",
			expectedGroupsCount:       3,
			expectedGroupOwnersCount:  1, // only alice is removed
			expectedGroupMembersCount: 0,
		},
		{
			name: "Group deleted",
			groupDelta: &models.ListGroupsDeltaResponse{
				Group: &models.Group{
					DirectoryObject: models.DirectoryObject{
						ID: to.Ptr(group3),
					},
				},
				Removed: &models.RemovedReason{
					Reason: to.Ptr("deleted"),
				},
			},
			expectedDisplay:           "group3",
			expectedGroupsCount:       2,
			expectedGroupOwnersCount:  0,
			expectedGroupMembersCount: 0,
			isRemoved:                 true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filtermatcher := groupFilterMatcher(filter.Filters{
				{Include: &types.PluginSyncFilter_NameRegex{NameRegex: "*"}},
			})

			// Prepare Entra id user, group and members.
			entraUsers := map[entraUniqueID]types.User{
				entraUniqueID(alice): mustTeleportUser("alice", alice),
				entraUniqueID(bob):   mustTeleportUser("bob", bob),
				entraUniqueID(carol): mustTeleportUser("carol", carol),
			}
			entraGroup1 := newEntraGroup(t, group1, "group1")
			entraGroup1.Owners = []*models.User{entraUser(t, alice, "alice"), entraUser(t, carol, "carol")}
			entraGroup2 := newEntraGroup(t, group2, "group2")
			entraGroup2.Owners = []*models.User{entraUser(t, alice, "alice"), entraUser(t, carol, "carol")}
			entraGroup3 := newEntraGroup(t, group3, "group3") // has no owners
			groupsMap := groupsByID{
				group1: entraGroup1,
				group2: entraGroup2,
				group3: entraGroup3,
			}

			groupMembersMap := groupMembersByGroupID{
				group1: {entraUser(t, alice, "alice"), newEntraGroup(t, group2, "group2")},
				group2: {entraUser(t, alice, "alice"), entraUser(t, carol, "carol")},
				group3: {entraUser(t, alice, "alice"), entraUser(t, bob, "bob")},
			}
			entraGroupMaps := entraGroups{
				groupsMap:       groupsMap,
				groupMembersMap: groupMembersMap,
			}
			ownerConfig := aclOwnersConfig{
				defaultOwners: []accesslist.Owner{{Name: "owner"}},
				source:        types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_ENTRAID,
			}
			// Convert Entra user, group and members to Teleport access list.
			// This mimics existing Entra resource already synced to Teleport,
			// i.e. after a first full sync.
			teleportALMMap, _ := entraGroupMaps.toAccessListsWithMembers(ctx, tenantID, entraUsers, ownerConfig)
			teleportUsers := make(map[string]types.User)
			for _, v := range entraUsers {
				teleportUsers[v.GetName()] = v
			}

			cfg := groupDeltaProcessorConfig{
				matcher:             filtermatcher,
				accessListsMap:      teleportALMMap,
				teleportUsersMap:    teleportUsers,
				setEntraGroupOwners: true,
				log:                 logtest.NewLogger(),
			}
			deltaProcessor := newGroupsDeltaProcessor(ctx, cfg)
			err := deltaProcessor.apply(ctx, tc.groupDelta)
			require.NoError(t, err)

			out := deltaProcessor.result()
			require.Len(t, out.groupsMap, tc.expectedGroupsCount, "expected groups map to be equal")
			require.Len(t, out.groupMembersMap[entraUniqueID(*tc.groupDelta.ID)], tc.expectedGroupMembersCount, "expected groups members map to be equal")

			if tc.isRemoved {
				return
			}
			require.Len(t, out.groupsMap[entraUniqueID(*tc.groupDelta.ID)].Owners, tc.expectedGroupOwnersCount, "expected groups owners map length to match")
			require.Equal(t, tc.expectedDisplay, *out.groupsMap[entraUniqueID(*tc.groupDelta.ID)].DisplayName, "expected group display name to match")
		})
	}
}

func TestGroupOwnerBuilder(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	teleportUsers := map[string]types.User{
		"alice": mustTeleportUser("alice", alice),
		"bob":   mustTeleportUser("bob", bob),
	}

	acl, err := accesslist.NewAccessList(
		header.Metadata{
			Name: group1,
			Labels: map[string]string{
				types.EntraUniqueIDLabel: group1,
			},
		},
		accesslist.Spec{
			Title: group1,
			Owners: []accesslist.Owner{
				{
					Name: "alice",
				},
				{
					Name: "bob",
				},
			},
		},
	)
	require.NoError(t, err)

	for _, expectOwners := range []bool{true, false} {
		t.Run(fmt.Sprintf("expectOwners=%s", strconv.FormatBool(expectOwners)), func(t *testing.T) {
			builder := groupBaseBuilder{
				accessLists: map[string]*accessListWithMembers{
					group1: &accessListWithMembers{
						AccessList: acl,
						Members:    make([]*accesslist.AccessListMember, 0),
					},
				},
				users:               teleportUsers,
				setEntraGroupOwners: expectOwners,
				log:                 logtest.NewLogger(),
			}
			base := builder.build(ctx)

			if expectOwners {
				require.Len(t, base.groupOwnersMap[group1], 2, "expected groupOwnersMap count mismatch")
				require.Contains(t, base.groupOwnersMap[group1], entraUniqueID(alice))
				require.Contains(t, base.groupOwnersMap[group1], entraUniqueID(bob))
			} else {
				require.Empty(t, base.groupOwnersMap[group1], "groupOwnersMap not empty when owners source is plugin")
			}
		})
	}
}

// mustTeleportUser creates user with entra id origin.
func mustTeleportUser(name string, entraUserID string) types.User {
	user, err := types.NewUser(name)
	if err != nil {
		panic(err)
	}

	labels := make(map[string]string)
	labels[types.EntraUniqueIDLabel] = entraUserID
	labels[types.EntraUPNLabel] = name

	user.SetStaticLabels(labels)
	user.SetOrigin(types.OriginEntraID)
	user.SetTraits(make(map[string][]string))

	return user
}

const (
	tenantID    = "abc-tenant"
	connectorID = "abc-connector"

	alice = "94f55977-96dc-48a7-9011-8c51e6114d42"
	bob   = "b639d90d-7414-43ff-a797-7bf4ad0c3a14"
	carol = "ce7d114c-9fa6-432a-9882-7a7421b2d486"

	group1 = "8fcd4cd0-bc00-4146-b38b-706f2f92d609"
	group2 = "f1f6118f-0b61-4e83-9b97-48a1c0c03269"
	group3 = "9fe8b5d4-cd39-4b29-bae5-67d328a1910b"
)
