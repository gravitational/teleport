package entraid

import (
	"fmt"
	"maps"
	"net/http"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/e/lib/entraid"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/lib/msgraph/models"
	"github.com/gravitational/teleport/lib/msgraph/msgraphtest"
)

func TestResourceImportDelta(t *testing.T) {
	ctx := t.Context()

	defaultStorage := newDefaultStorage()
	clock := clockwork.NewFakeClock()
	env := newTestEnv(t, defaultStorage, common.WithClock(clock))

	// Install the plugin, should trigger resource import on first start.
	plugin := newDefaultPluginSpec(t)
	settings := plugin.Spec.GetEntraId().SyncSettings
	settings.SyncIntervals = &types.PluginEntraIDSyncIntervals{
		Delta: "15s",
		Full:  "1h",
	}
	plugin.Spec.Settings = &types.PluginSpecV1_EntraId{
		EntraId: &types.PluginEntraIDSettings{
			SyncSettings: settings,
		},
	}

	err := createEntraIDPlugin(ctx, env.authClient, plugin)
	require.NoError(t, err, "expected Entra ID plugin to be created")

	// First full sync, should import all the user, group and group membership
	// defined in [newDefaultStorage].
	expectPluginStatusUpdated(t, ctx, env.authClient, plugin.GetName(), clock)
	expectDefaultPluginStatus(t, env.authClient, plugin.GetName())
	expectDefaultUserSync(t, env.authClient)
	expectDefaultGroupSync(t, env.authClient)
	expectDefaultGroupOwners(t, env.authClient)

	// Wait for the next sync, which is the first delta sync.
	// Items synced with first full sync should remain unchanged.
	expectPluginStatusUpdated(t, ctx, env.authClient, plugin.GetName(), clock)
	expectDefaultUserSync(t, env.authClient)
	expectDefaultGroupSync(t, env.authClient)
	expectDefaultGroupOwners(t, env.authClient)

	authClt := env.authClient
	t.Run("Add and delete user", func(t *testing.T) {
		aliceN := newEntraUser("848f8e0c-2714-4745-b478-1f7b59620ee5", "aliceN@example.com", "Alice N")
		env.fakeServer.SetUsers([]*models.User{aliceN})
		env.fakeServer.SetGroupMembers(group1ID, []models.GroupMember{aliceN})
		env.fakeServer.DeleteUsers([]string{aliceID})
		// Wait for the new changes to be discovered with delta sync.
		expectPluginStatusUpdated(t, ctx, env.authClient, plugin.GetName(), clock)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			requireEntraIDUsers(t, ctx, env.authClient, []string{
				"bob@example.com",
				"carol@example.com",
				// alice was deleted
				"aliceN@example.com",
			})
		}, 15*time.Second, 30*time.Millisecond)

		// Changes in group membership should update user traits.
		requireRoleAndTraits(t, ctx, authClt, "bob@example.com", []string{"access", "requester"}, []string{group1ID, group2ID, group3ID}, "expected bob role traits to be updated")
		requireRoleAndTraits(t, ctx, authClt, "aliceN@example.com", []string{"requester"}, []string{group1ID}, "expected aiceN traits to be updated")

		// Expect user alice to not be present in access list memebership.
		require.EventuallyWithT(t,
			func(t *assert.CollectT) {
				// Group names matches with default payload available in newDefaultStorage().
				expectedAccessListTitles := []string{"group1", "group2", "group3"}
				gotAccesslists, err := listEntraIDAccessLists(ctx, env.authClient.AccessListClient())
				require.NoError(t, err)
				require.NotNil(t, gotAccesslists)
				require.ElementsMatch(t, expectedAccessListTitles, slices.Collect(maps.Keys(gotAccesslists)), "expected Entra ID groups to be created")

				requireEntraIDAccessListMembers(t, ctx, authClt,
					gotAccesslists["group1"],
					[]string{gotAccesslists["group2"].GetName(), "aliceN@example.com"}, // group2 is nested member of group1.
				)

				requireEntraIDAccessListMembers(t, ctx, authClt,
					gotAccesslists["group2"],
					[]string{"bob@example.com", "carol@example.com"},
				)

				requireEntraIDAccessListMembers(t, ctx, authClt,
					gotAccesslists["group3"],
					[]string{"bob@example.com", "carol@example.com"},
				)

			},
			time.Second*10, time.Millisecond*30)
	})

	t.Run("Add and delete Group", func(t *testing.T) {
		group4 := newEntraGroup("e12d20ab-0104-495e-bc24-75989f88f590", "group4")
		env.fakeServer.SetGroups([]*models.Group{group4})
		// delete group3
		env.fakeServer.DeleteGroups([]string{group3ID})
		expectPluginStatusUpdated(t, ctx, env.authClient, plugin.GetName(), clock)
		require.EventuallyWithT(t,
			func(t *assert.CollectT) {
				// Group names matches with default payload available in newDefaultStorage().
				expectedAccessListTitles := []string{"group1", "group2", "group4"}
				gotAccesslists, err := listEntraIDAccessLists(ctx, env.authClient.AccessListClient())
				require.NoError(t, err)
				require.NotNil(t, gotAccesslists)
				require.ElementsMatch(t, expectedAccessListTitles, slices.Collect(maps.Keys(gotAccesslists)), "expected Entra ID groups to be created")
			},
			time.Second*30, time.Millisecond*30)

		// group3 was deleted and bob should no longer have this group trait.
		// group3 grants "access" role.
		requireRoleAndTraits(t, ctx, authClt, "bob@example.com", []string{"requester"}, []string{group1ID, group2ID}, "expected access role and group3 trait to be removed")
	})
}

// TestUserGroupTraitGrantsInDeltaSync tests that user's group membership in
// office 365 groups should not be added to user's group trait.
func TestUserGroupTraitGrantsInDeltaSync(t *testing.T) {
	ctx := t.Context()
	defaultStorage := newDefaultStorage()

	// User: alice.
	// Groups: group1, group2, o365Group1, o365Group2, o365Group3.
	// Group membership:
	//   Alice as a:
	//   - direct member of group1, o365Group1, o365Group3.
	//   - indirect member of group2 and o365Group2.
	storage := msgraphtest.NewStorage()
	storage.Applications = defaultStorage.Applications

	// User
	storage.Users = make(map[string]*models.User)
	storage.Users[aliceID] = defaultStorage.Users[aliceID]

	// Groups
	storage.Groups = make(map[string]*models.Group)
	storage.Groups[group1ID] = defaultStorage.Groups[group1ID]
	storage.Groups[group2ID] = defaultStorage.Groups[group2ID]

	const o365Group1ID = "6a2c61d0-9519-402d-9f0e-37f76590d8b2"
	o365Group1 := newEntraGroup(o365Group1ID, "o365Group1")
	o365Group1.GroupTypes = []string{"Unified"} // Unified signifies office 365 group.
	storage.Groups[o365Group1ID] = o365Group1

	const o365Group2ID = "47c5e3eb-a0c7-4d17-b00f-993a2682d4fd"
	o365Group2 := newEntraGroup(o365Group2ID, "o365Group2")
	o365Group2.GroupTypes = []string{"Unified"}
	storage.Groups[o365Group2ID] = o365Group2

	const o365Group3ID = "549e7c31-e5df-455c-8e8f-406284cac2e6"
	o365Group3 := newEntraGroup(o365Group3ID, "o365Group3")
	o365Group3.GroupTypes = []string{"Unified"}
	storage.Groups[o365Group3ID] = o365Group3

	// Group membership
	alice := defaultStorage.Users[aliceID]
	storage.GroupMembers[group1ID] = []models.GroupMember{alice}
	storage.GroupMembers[group2ID] = []models.GroupMember{o365Group1}
	storage.GroupMembers[o365Group1ID] = []models.GroupMember{alice}
	storage.GroupMembers[o365Group2ID] = []models.GroupMember{o365Group3}
	storage.GroupMembers[o365Group3ID] = []models.GroupMember{alice}

	clock := clockwork.NewFakeClock()
	env := newTestEnv(t, storage, common.WithClock(clock))
	authClient := env.authClient

	// Install the plugin, should trigger resource import on first start.
	plugin := newDefaultPluginSpec(t)
	settings := plugin.Spec.GetEntraId().SyncSettings
	settings.SyncIntervals = &types.PluginEntraIDSyncIntervals{
		Delta: "15s",
		Full:  "1h",
	}
	plugin.Spec.Settings = &types.PluginSpecV1_EntraId{
		EntraId: &types.PluginEntraIDSettings{
			SyncSettings: settings,
		},
	}
	err := createEntraIDPlugin(ctx, authClient, plugin)
	require.NoError(t, err, "expected Entra ID plugin to be created")

	aliceMembership := []string{"alice@example.com"}
	expectedAccessListTitles := []string{"group1", "group2", "o365Group1", "o365Group2", "o365Group3"}
	expectedGroupTraits := []string{group1ID}
	expectUser := func(t *testing.T) {
		t.Helper()
		require.EventuallyWithT(t,
			func(t *assert.CollectT) {
				requireEntraIDUsers(t, ctx, authClient, []string{"alice@example.com"})
			},
			time.Second*15, time.Millisecond*30)
	}
	expectGroupAndMembership := func(t *testing.T) {
		t.Helper()
		require.EventuallyWithT(t,
			func(t *assert.CollectT) {
				gotAccesslists, err := listEntraIDAccessLists(ctx, authClient.AccessListClient())
				require.NoError(t, err)
				require.NotNil(t, gotAccesslists)
				require.ElementsMatch(t, expectedAccessListTitles, slices.Collect(maps.Keys(gotAccesslists)), "expected Entra ID groups to be created")

				requireEntraIDAccessListMembers(t, ctx, authClient, gotAccesslists["group1"], aliceMembership)
				requireEntraIDAccessListMembers(t, ctx, authClient, gotAccesslists["group2"], []string{gotAccesslists["o365Group1"].GetName()})
				requireEntraIDAccessListMembers(t, ctx, authClient, gotAccesslists["o365Group1"], aliceMembership)
				requireEntraIDAccessListMembers(t, ctx, authClient, gotAccesslists["o365Group2"], []string{gotAccesslists["o365Group3"].GetName()})
				requireEntraIDAccessListMembers(t, ctx, authClient, gotAccesslists["o365Group3"], aliceMembership)
			},
			time.Second*15, time.Millisecond*30)
	}

	// First full sync, should import all the user, group and group membership
	// defined in [newDefaultStorage].
	t.Run("First full sync", func(t *testing.T) {
		expectPluginStatusUpdated(t, ctx, env.authClient, plugin.GetName(), clock)
		expectUser(t)
		expectGroupAndMembership(t)
		// Trait value representing for o365Group1, o365Group2 and o365Group3 should have been filtered out.
		requireRoleAndTraits(t, ctx, authClient, "alice@example.com", []string{"requester"}, expectedGroupTraits, "expected alice traits to match")
	})

	t.Run("First delta sync", func(t *testing.T) {
		// Wait for the next sync, which is the first delta sync.
		// Items synced with first full sync should remain unchanged.
		expectPluginStatusUpdated(t, ctx, env.authClient, plugin.GetName(), clock)
		expectUser(t)
		expectGroupAndMembership(t)
		requireRoleAndTraits(t, ctx, authClient, "alice@example.com", []string{"requester"}, expectedGroupTraits, "expected alice traits to match")
	})

	// Mimic delta sync with two groups - one security and one O365 group
	// are added where alice is member of both the groups.
	// Alice should be granted with a new group trait that matches non-o365 group.
	t.Run("Second delta sync with a new security and O365 group with membership", func(t *testing.T) {
		const group3ID = "e12d20ab-0104-495e-bc24-75989f88f590"
		group3 := newEntraGroup(group3ID, "group3")
		const o365Group4ID = "ab617c7d-674d-429e-843c-38c0ee762013"
		o365Group4 := newEntraGroup(o365Group4ID, "o365Group4")
		o365Group4.GroupTypes = []string{"Unified"}
		env.fakeServer.SetGroups([]*models.Group{group3, o365Group4})

		env.fakeServer.SetGroupMembers(group3ID, []models.GroupMember{alice})
		env.fakeServer.SetGroupMembers(o365Group4ID, []models.GroupMember{alice})

		// Wait for plugin status to be updated.
		expectPluginStatusUpdated(t, ctx, env.authClient, plugin.GetName(), clock)

		expectedAccessListTitlesAfterDelta := slices.Concat(expectedAccessListTitles, []string{"group3", "o365Group4"})
		require.EventuallyWithT(t,
			func(t *assert.CollectT) {
				gotAccesslists, err := listEntraIDAccessLists(ctx, env.authClient.AccessListClient())
				require.NoError(t, err)
				require.NotNil(t, gotAccesslists)
				require.ElementsMatch(t, expectedAccessListTitlesAfterDelta, slices.Collect(maps.Keys(gotAccesslists)), "expected Entra ID groups to be created")

				requireEntraIDAccessListMembers(t, ctx, authClient, gotAccesslists["group1"], aliceMembership)
				requireEntraIDAccessListMembers(t, ctx, authClient, gotAccesslists["group2"], []string{gotAccesslists["o365Group1"].GetName()})
				requireEntraIDAccessListMembers(t, ctx, authClient, gotAccesslists["group3"], aliceMembership)
				requireEntraIDAccessListMembers(t, ctx, authClient, gotAccesslists["o365Group1"], aliceMembership)
				requireEntraIDAccessListMembers(t, ctx, authClient, gotAccesslists["o365Group2"], []string{gotAccesslists["o365Group3"].GetName()})
				requireEntraIDAccessListMembers(t, ctx, authClient, gotAccesslists["o365Group3"], aliceMembership)
				requireEntraIDAccessListMembers(t, ctx, authClient, gotAccesslists["o365Group4"], aliceMembership)
			},
			time.Second*30, time.Millisecond*30)

		// Trait value representing for o365Group1, o365Group2, o365Group3 and o365Group4 should have been filtered out.
		expectedGroupTraits = []string{group1ID, group3ID}
		requireRoleAndTraits(t, ctx, authClient, "alice@example.com", []string{"requester"}, expectedGroupTraits, "expected alice traits to match")
	})

	t.Run("Subsequent delta sync", func(t *testing.T) {
		// Double check trait remains the same on a subsequent noop delta sync.
		expectPluginStatusUpdated(t, ctx, env.authClient, plugin.GetName(), clock)
		requireRoleAndTraits(t, ctx, authClient, "alice@example.com", []string{"requester"}, expectedGroupTraits, "expected alice traits to match")
	})
}

// TestDeltaSyncWorksOnSoftErrors tests that delta service is
// run when service hits soft errors i.e. known errors that are
// skipped while processing Entra ID resources.
func TestDeltaSyncWorksOnSoftErrors(t *testing.T) {
	ctx := t.Context()

	defaultStorage := newDefaultStorage()
	// Unsupported user with quote "'" in username.
	davidInvalid := newEntraUser("f25cfcf2-913d-4fd0-a2d3-4d7e7ac2100b", "dav'id@example.com", "David D")
	// Add new users david to the default storage users,
	// Default users alice, bob and carol remains unchanged.
	defaultStorage.Users[*davidInvalid.ID] = davidInvalid
	// Add davidInvalid as a member to group1.
	defaultStorage.GroupMembers[group1ID] = slices.Concat(defaultStorage.GroupMembers[group1ID], []models.GroupMember{davidInvalid})
	clock := clockwork.NewFakeClock()
	env := newTestEnv(t, defaultStorage, common.WithClock(clock))

	plugin := newDefaultPluginSpec(t)
	settings := plugin.Spec.GetEntraId().SyncSettings
	settings.SyncIntervals = &types.PluginEntraIDSyncIntervals{
		Delta: "15s",
		Full:  "1h",
	}
	plugin.Spec.Settings = &types.PluginSpecV1_EntraId{
		EntraId: &types.PluginEntraIDSettings{
			SyncSettings: settings,
		},
	}
	// Install the plugin, should trigger resource import on first start.
	err := createEntraIDPlugin(ctx, env.authClient, plugin)
	require.NoError(t, err, "expected Entra ID plugin to be created")

	// First full sync, expect no errors.
	expectPluginStatusUpdated(t, ctx, env.authClient, plugin.GetName(), clock)
	expectDefaultUserSync(t, env.authClient)
	expectDefaultGroupSync(t, env.authClient)
	expectDefaultGroupOwners(t, env.authClient)

	// Wait for the next sync, which is the first delta sync.
	// Proves that delta sync was reached when unsupported account
	// was encountered.
	expectPluginStatusUpdated(t, ctx, env.authClient, plugin.GetName(), clock)
	expectDefaultUserSync(t, env.authClient)
	expectDefaultGroupSync(t, env.authClient)
	expectDefaultGroupOwners(t, env.authClient)

	t.Run("Add user", func(t *testing.T) {
		// Check delta sync with unsupported user.
		aliceN := newEntraUser("848f8e0c-2714-4745-b478-1f7b59620ee5", "aliceN@example.com", "Alice N")
		env.fakeServer.SetUsers([]*models.User{aliceN, davidInvalid})
		// Wait for the new changes to be discovered with delta sync.
		expectPluginStatusUpdated(t, ctx, env.authClient, plugin.GetName(), clock)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			requireEntraIDUsers(t, ctx, env.authClient, []string{
				"alice@example.com",
				"bob@example.com",
				"carol@example.com",
				"aliceN@example.com",
			})
		}, 15*time.Second, 30*time.Millisecond)
	})
}

func TestGroupOwnersDelta(t *testing.T) {
	ctx := t.Context()

	defaultStorage := newDefaultStorage()
	alice := defaultStorage.Users[aliceID]
	bob := defaultStorage.Users[bobID]
	carol := defaultStorage.Users[carolID]

	clock := clockwork.NewFakeClock()
	env := newTestEnv(t, defaultStorage, common.WithClock(clock))

	// Install the plugin, should trigger resource import on first start.
	plugin := newDefaultPluginSpec(t)
	settings := plugin.Spec.GetEntraId().SyncSettings
	settings.SyncIntervals = &types.PluginEntraIDSyncIntervals{
		Delta: "15s",
		Full:  "30m",
	}
	settings.AccessListOwnersSource = types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_ENTRAID
	plugin.Spec.Settings = &types.PluginSpecV1_EntraId{
		EntraId: &types.PluginEntraIDSettings{
			SyncSettings: settings,
		},
	}
	err := createEntraIDPlugin(ctx, env.authClient, plugin)
	require.NoError(t, err, "expected Entra ID plugin to be created")

	// First full sync.
	expectPluginStatusUpdated(t, ctx, env.authClient, plugin.GetName(), clock)
	expectDefaultPluginStatus(t, env.authClient, plugin.GetName())
	expectDefaultUserSync(t, env.authClient)
	expectDefaultGroupSync(t, env.authClient)

	accessListClient := env.authClient.AccessListClient()

	// Names matches with default group payload available in defaultStorage.
	expectedAccessListTitles := []string{"group1", "group2", "group3"}
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			gotAccesslists, err := listEntraIDAccessLists(ctx, accessListClient)
			require.NoError(t, err)
			require.ElementsMatch(t, expectedAccessListTitles, slices.Collect(maps.Keys(gotAccesslists)), "expected Entra ID groups to be created")

			// alice and bob are default owners configured in defaultStorage.
			expectedGroup1Owners := aclOwners(t, []*models.User{alice, bob})
			compareOwners(t, "group1", expectedGroup1Owners, gotAccesslists["group1"].Spec.Owners)

			// group2 has zero owners, should fallback to default owners.
			compareOwners(t, "group2", []accesslist.Owner{defaultOwner}, gotAccesslists["group2"].Spec.Owners)

			// bob and carol are default owners configured in defaultStorage.
			expectedGroup3Owners := aclOwners(t, []*models.User{bob, carol})
			compareOwners(t, "group3", expectedGroup3Owners, gotAccesslists["group3"].Spec.Owners)
		},
		time.Second*10, time.Millisecond*30)

	// First delta sync
	expectPluginStatusUpdated(t, ctx, env.authClient, plugin.GetName(), clock)
	expectDefaultUserSync(t, env.authClient)
	expectDefaultGroupSync(t, env.authClient)

	t.Run("Add and delete group owner", func(t *testing.T) {
		// Remove carol from group3 ownership
		env.fakeServer.DeleteGroupOwners(group3ID, []string{carolID})
		// Add alice to group3 ownership
		env.fakeServer.SetGroupOwners(group3ID, []*models.User{alice})

		expectPluginStatusUpdated(t, ctx, env.authClient, plugin.GetName(), clock)

		require.EventuallyWithT(t,
			func(t *assert.CollectT) {
				gotAccesslists, err := listEntraIDAccessLists(ctx, accessListClient)
				require.NoError(t, err)
				require.ElementsMatch(t, expectedAccessListTitles, slices.Collect(maps.Keys(gotAccesslists)), "expected Entra ID groups to be created")

				// Group1 ownership remains unchanged
				expectedGroup1Owners := aclOwners(t, []*models.User{alice, bob})
				compareOwners(t, "group1", expectedGroup1Owners, gotAccesslists["group1"].Spec.Owners)

				// group2 has zero owners, should fallback to default owners.
				compareOwners(t, "group2", []accesslist.Owner{defaultOwner}, gotAccesslists["group2"].Spec.Owners)

				// carol was removed, should contain alice and bob.
				expectedGroup3Owners := aclOwners(t, []*models.User{alice, bob})
				compareOwners(t, "group3", expectedGroup3Owners, gotAccesslists["group3"].Spec.Owners)
			},
			time.Second*10, time.Millisecond*30)
	})
}

func TestResourceImportWithCyclicGroupMembersDelta(t *testing.T) {
	ctx := t.Context()

	defaultStorage := newDefaultStorage()
	alice := defaultStorage.Users[aliceID]
	bob := defaultStorage.Users[bobID]
	group1 := defaultStorage.Groups[group1ID]
	group2 := defaultStorage.Groups[group2ID]
	group3 := defaultStorage.Groups[group3ID]

	defaultStorage.GroupMembers = make(map[string][]models.GroupMember)
	defaultStorage.GroupMembers[*group1.GetID()] = []models.GroupMember{alice}
	defaultStorage.GroupMembers[*group2.GetID()] = []models.GroupMember{group1, alice}
	defaultStorage.GroupMembers[*group3.GetID()] = []models.GroupMember{group1, alice, bob}

	clock := clockwork.NewFakeClock()
	env := newTestEnv(t, defaultStorage, common.WithClock(clock))

	// Install the plugin, should trigger resource import on first start.
	plugin := newDefaultPluginSpec(t)
	settings := plugin.Spec.GetEntraId().SyncSettings
	settings.SyncIntervals = &types.PluginEntraIDSyncIntervals{
		Delta: "15s",
		Full:  "1h",
	}
	plugin.Spec.Settings = &types.PluginSpecV1_EntraId{
		EntraId: &types.PluginEntraIDSettings{
			SyncSettings: settings,
		},
	}

	err := createEntraIDPlugin(ctx, env.authClient, plugin)
	require.NoError(t, err, "expected Entra ID plugin to be created")

	// First full sync, should import all the user, group and group membership
	// defined in [newDefaultStorage].
	expectPluginStatusUpdated(t, ctx, env.authClient, plugin.GetName(), clock)
	expectDefaultPluginStatus(t, env.authClient, plugin.GetName())
	accessListClient := env.authClient.AccessListClient()
	expectDefaultUserSync(t, env.authClient)
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			// No membership that introduces cycle and all members are expected to be imported.

			gotAccesslists, err := listEntraIDAccessLists(ctx, accessListClient)
			require.NoError(t, err)
			require.ElementsMatch(t, defaultGroupDisplayNames, slices.Collect(maps.Keys(gotAccesslists)), "expected Entra ID groups to be created")

			group1members := mustListMembers(t, ctx, gotAccesslists["group1"].GetName(), accessListClient)
			require.ElementsMatch(t, []string{"alice@example.com"}, group1members, "expected Entra group1 members to match")

			group2Members := mustListMembers(t, ctx, gotAccesslists["group2"].GetName(), accessListClient)
			require.ElementsMatch(t, []string{gotAccesslists["group1"].GetName(), "alice@example.com"}, group2Members, "expected Entra group2 members to match")

			group3Members := mustListMembers(t, ctx, gotAccesslists["group3"].GetName(), accessListClient)
			require.ElementsMatch(t, []string{gotAccesslists["group1"].GetName(), "alice@example.com", "bob@example.com"}, group3Members, "expected Entra group3 members to match")
		},
		time.Second*20, time.Millisecond*30)

	// group1 is already a member of group2.
	// Now add group2 as a member of group1, creating cyclic relationship.
	env.fakeServer.SetGroupMembers(group1ID, []models.GroupMember{group2})
	expectPluginStatusUpdated(t, ctx, env.authClient, plugin.GetName(), clock)
	expectDefaultUserSync(t, env.authClient)
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			gotAccesslists, err := listEntraIDAccessLists(ctx, accessListClient)
			require.NoError(t, err)
			require.ElementsMatch(t, []string{"group1", "group2", "group3"}, slices.Collect(maps.Keys(gotAccesslists)), "expected Entra ID groups to be created")

			// Cyclic membership removal is of sorted order, and existing edge wins.
			// Since group2 -> group1 already exists, the new edge group1 -> group2 which
			// introduces cycle must be removed.
			group1members := mustListMembers(t, ctx, gotAccesslists["group1"].GetName(), accessListClient)
			require.Len(t, group1members, 1)

			// No changes expected in group2.
			group2Members := mustListMembers(t, ctx, gotAccesslists["group2"].GetName(), accessListClient)
			require.Len(t, group2Members, 2)

			// No changes expected in group3.
			group3Members := mustListMembers(t, ctx, gotAccesslists["group3"].GetName(), accessListClient)
			require.ElementsMatch(t, []string{gotAccesslists["group1"].GetName(), "alice@example.com", "bob@example.com"}, group3Members, "expected Entra group3 members to match")
		},
		time.Second*20, time.Millisecond*30)
}

// TestResourcesUnchangedOnNoopDeltaSync checks that a Noop delta sync,
// i.e. no changes tracked should not have any backend writes.
func TestResourcesUnchangedOnNoopDeltaSync(t *testing.T) {
	ctx := t.Context()

	// Setup.
	defaultStorage := newDefaultStorage()
	clock := clockwork.NewFakeClock()
	env := newTestEnv(t, defaultStorage, common.WithClock(clock))
	w := env.sut.NewResourceWatcher(t, types.KindPlugin)
	defer w.Close()

	mustCreatePlugin(t,
		ctx,
		env.authClient,
		withSyncIntervals(&types.PluginEntraIDSyncIntervals{
			Delta: deltaSyncInterval.String(),
			Full:  "0", // full sync disabled
		}),
		// Owners source to be plugin and Entra ID.
		withAclOwnersConfig(types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_PLUGIN_AND_ENTRAID),
	)

	// First full sync, should import all the user, group and group membership
	// defined in [newDefaultStorage].
	lastSyncTime := waitForPluginStatusUpdate(t, w, time.Time{} /* last sync time empty on first sync */)
	expectDefaultResourcesWithPluginAndEntraOwners(t, ctx, env.authClient)

	// Test that the resource revisions from the first sync matches with
	// resource revisions in subsequent delta sync.

	// Get revision of users and access lists.
	userRevisionsOnFirstSync := mustGetUserRevisions(t, ctx, env.authClient, defaultUsernames)
	require.Len(t, userRevisionsOnFirstSync, 3)
	aclNames := mustGetAclNames(t, ctx, env.authClient.AccessListClient(), defaultGroupDisplayNames)
	aclRevisionsOnFullSync := mustGetAclWithMembersRevision(t, ctx, env.authClient, aclNames)

	// Delta syncs.
	for _, v := range []string{"First", "Second", "Third"} {
		t.Run(fmt.Sprintf("%s delta sync", v), func(t *testing.T) {
			clock.Advance(deltaSyncInterval)
			lastSyncTime = waitForPluginStatusUpdate(t, w, lastSyncTime)

			userRevisionsAfter := mustGetUserRevisions(t, ctx, env.authClient, defaultUsernames)
			require.ElementsMatch(t, userRevisionsOnFirstSync, userRevisionsAfter, "user revision changed")

			aclRevisionsAfter := mustGetAclWithMembersRevision(t, ctx, env.authClient, aclNames)
			require.ElementsMatch(t, aclRevisionsOnFullSync, aclRevisionsAfter, "acl revision changed")
		})
	}
}

func TestOnlyTrackedResourceIsUpdated_UserDelta(t *testing.T) {
	ctx := t.Context()

	// Setup.
	defaultStorage := newDefaultStorage()
	clock := clockwork.NewFakeClock()
	env := newTestEnv(t, defaultStorage, common.WithClock(clock))
	pluginWatcher := env.sut.NewResourceWatcher(t, types.KindPlugin)
	defer pluginWatcher.Close()
	userWatcher := env.sut.NewResourceWatcher(t, types.KindUser)
	defer userWatcher.Close()

	mustCreatePlugin(t,
		ctx,
		env.authClient,
		withSyncIntervals(&types.PluginEntraIDSyncIntervals{
			Delta: deltaSyncInterval.String(),
			Full:  "0", // full sync disabled
		}),
	)

	// First full sync, should import all the user, group and group membership
	// defined in [newDefaultStorage].
	lastSyncTime := waitForPluginStatusUpdate(t, pluginWatcher, time.Time{} /* last sync time empty on first sync */)
	expectDefaultResources(t, env.authClient)

	// Get revision of users and access lists.
	usernames := []string{"alice@example.com", "carol@example.com"}
	userRevisionsOnFirstSync := mustGetUserRevisions(t, ctx, env.authClient, usernames)

	aclNames := mustGetAclNames(t, ctx, env.authClient.AccessListClient(), defaultGroupDisplayNames)
	aclRevisionsOnFullSync := mustGetAclWithMembersRevision(t, ctx, env.authClient, aclNames)
	// Sanity check on expected acl and member counts.
	require.Len(t, aclRevisionsOnFullSync, 3)
	require.NotEmpty(t, aclRevisionsOnFullSync[0].MembersRevision)
	require.NotEmpty(t, aclRevisionsOnFullSync[1].MembersRevision)
	require.NotEmpty(t, aclRevisionsOnFullSync[2].MembersRevision)

	// Update Bob display
	bobUpdatedDisplay := "New Bob"
	bob := defaultStorage.Users[bobID]
	bob.DisplayName = to.Ptr(bobUpdatedDisplay)

	// Creates a delta diff for Bob.
	env.fakeServer.SetUsers([]*models.User{bob})

	// First delta sync.
	clock.Advance(deltaSyncInterval)
	waitForPluginStatusUpdate(t, pluginWatcher, lastSyncTime)
	expectDefaultResources(t, env.authClient)

	userRevisionsAfter := mustGetUserRevisions(t, ctx, env.authClient, usernames)
	require.ElementsMatch(t, userRevisionsOnFirstSync, userRevisionsAfter, "user revision changed")
	common.WaitForPutEvent(t, userWatcher, func(r types.User) bool {
		displayTrait := "http://schemas.microsoft.com/identity/claims/displayname"
		return r.GetName() == "bob@example.com" && r.GetTraits()[displayTrait][0] == bobUpdatedDisplay
	})
	aclWithMembersAfter := mustGetAclWithMembersRevision(t, ctx, env.authClient, aclNames)
	require.Empty(t, cmp.Diff(aclRevisionsOnFullSync, aclWithMembersAfter))
}

func TestOnlyTrackedResourceIsUpdated_GroupDelta(t *testing.T) {
	ctx := t.Context()

	// Setup.
	defaultStorage := newDefaultStorage()
	clock := clockwork.NewFakeClock()
	env := newTestEnv(t, defaultStorage, common.WithClock(clock))
	pluginWatcher := env.sut.NewResourceWatcher(t, types.KindPlugin)
	defer pluginWatcher.Close()

	mustCreatePlugin(t,
		ctx,
		env.authClient,
		withSyncIntervals(&types.PluginEntraIDSyncIntervals{
			Delta: deltaSyncInterval.String(),
			Full:  "0", // full sync disabled
		}),
	)

	// First full sync, should import all the user, group and group membership
	// defined in [newDefaultStorage].
	lastSyncTime := waitForPluginStatusUpdate(t, pluginWatcher, time.Time{} /* last sync time empty on first sync */)
	expectDefaultResources(t, env.authClient)

	// Get revision of users and access lists.
	userRevisionsOnFirstSync := mustGetUserRevisions(t, ctx, env.authClient, defaultUsernames)

	// group1 and group2 collected, group3 will be checked separately because it will be updated.
	group1Andgroup2 := mustGetAclNames(t, ctx, env.authClient.AccessListClient(), []string{"group1", "group2"})
	aclRevisionsOnFullSync := mustGetAclWithMembersRevision(t, ctx, env.authClient, group1Andgroup2)
	// Sanity check on expected acl and member count.
	require.Len(t, aclRevisionsOnFullSync, 2)
	require.NotEmpty(t, aclRevisionsOnFullSync[0].MembersRevision)
	require.NotEmpty(t, aclRevisionsOnFullSync[1].MembersRevision)

	const group3UpdatedDisplay = "group3-updated"
	// First delta sync.
	t.Run("Group display updated", func(t *testing.T) {
		group3 := defaultStorage.Groups[group3ID]
		group3.DisplayName = to.Ptr(group3UpdatedDisplay)

		aclWatcher := env.sut.NewResourceWatcher(t, types.KindAccessList)
		defer aclWatcher.Close()

		// Updating group3 also creates a delta diff for group3.
		env.fakeServer.SetGroups([]*models.Group{group3})
		clock.Advance(deltaSyncInterval)
		lastSyncTime = waitForPluginStatusUpdate(t, pluginWatcher, lastSyncTime)
		expectDefaultPluginStatus(t, env.authClient, pluginName)
		expectDefaultUserSync(t, env.authClient)

		userRevisionsAfter := mustGetUserRevisions(t, ctx, env.authClient, defaultUsernames)
		require.ElementsMatch(t, userRevisionsOnFirstSync, userRevisionsAfter, "user revision changed")

		// Wait for group3 Access List to be updated.
		// The display name of the group is used to generate Access List resource ID.
		// So a new display name causes exsting Access List and its members to be recreated.
		// This behavior is the same for both "full" and "delta" sync.
		group3NewName := ""
		common.WaitForPutEvent(t, aclWatcher, func(r *accesslist.AccessList) bool {
			if r.Spec.Title == group3UpdatedDisplay {
				group3NewName = r.GetName()
				return true
			}
			return false
		})
		// Get updated access group 3 from the backend.
		updatedGroup3, err := env.authClient.AccessListClient().GetAccessList(ctx, group3NewName)
		require.NoError(t, err, "expected updated Access List")

		// group3 membership also updated to point to new Access List resource ID.
		group3MembersAfter, err := listEntraIDMembers(ctx, updatedGroup3.GetName(), env.authClient.AccessListClient())
		require.NoError(t, err)

		// Expect updated group3 members to be the same.
		group3MembersAfterNames := slices.Collect(func(yield func(string) bool) {
			for _, u := range group3MembersAfter {
				if !yield(u.GetName()) {
					return
				}
			}
		})
		require.ElementsMatch(t, defaultUsernames, group3MembersAfterNames, "expected updated Access List members to match")

		// Check group1 and group2 Access List and its members remains unchanged.
		aclWithMembersAfter := mustGetAclWithMembersRevision(t, ctx, env.authClient, group1Andgroup2)
		require.Empty(t, cmp.Diff(aclRevisionsOnFullSync, aclWithMembersAfter))
	})

	// Second delta sync.
	t.Run("Group member removed", func(t *testing.T) {
		group3 := defaultStorage.Groups[group3ID]
		group3.DisplayName = to.Ptr(group3UpdatedDisplay)

		aliceBeforeMembershipRemoval, err := env.authClient.GetUser(ctx, "alice@example.com", false)
		require.NoError(t, err)
		groupTraitKey := "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups"
		require.True(t, slices.Contains(aliceBeforeMembershipRemoval.GetTraits()[groupTraitKey], group3ID))

		// Get user revisions for Bob and Carol which should be unchanged.
		usernames := []string{"bob@example.com", "carol@example.com"}
		bobAndCarolRevisionBefore := mustGetUserRevisions(t, ctx, env.authClient, usernames)

		// Create user watcher.
		userWatcher := env.sut.NewResourceWatcher(t, types.KindUser)
		defer userWatcher.Close()

		// Removing group member also creates a delta diff for group3.
		env.fakeServer.DeleteGroupMembers(group3ID, []string{aliceID})
		clock.Advance(deltaSyncInterval)
		lastSyncTime = waitForPluginStatusUpdate(t, pluginWatcher, lastSyncTime)
		expectDefaultPluginStatus(t, env.authClient, pluginName)

		// Check user trait has been updated.
		common.WaitForPutEvent(t, userWatcher, func(r types.User) bool {
			return r.GetName() == "alice@example.com" && !slices.Contains(r.GetTraits()[groupTraitKey], group3ID)
		})
		bobAndCrolRevisionAfter := mustGetUserRevisions(t, ctx, env.authClient, usernames)
		require.ElementsMatch(t, bobAndCarolRevisionBefore, bobAndCrolRevisionAfter, "user revision changed")

		// Get new access lists from the backend.
		// group3 membership also updated to point to new Access List resource ID.
		gotAccesslists, err := listEntraIDAccessLists(ctx, env.authClient.AccessListClient())
		require.NoError(t, err, "list entra access lists")
		group3After, err := env.authClient.AccessListClient().GetAccessList(ctx, gotAccesslists[group3UpdatedDisplay].GetName())
		require.NoError(t, err)

		require.Equal(t, uint32(2), *group3After.GetStatus().MemberCount, "expected group3 to be updated with new member count status")

		// Check group1 and group3 Access List and its members remains unchanged.
		aclWithMembersAfter := mustGetAclWithMembersRevision(t, ctx, env.authClient, group1Andgroup2)
		require.Empty(t, cmp.Diff(aclRevisionsOnFullSync, aclWithMembersAfter))
	})
}

// Group filter should be applied to delta sync.
// If a group display is updated, and the new display matches with exclude filter,
// corresponding Access List should be removed from Teleport.
func TestGroupFilterAppliesOnGroupDisplayUpdate(t *testing.T) {
	ctx := t.Context()

	// Setup.
	defaultStorage := newDefaultStorage()
	clock := clockwork.NewFakeClock()
	env := newTestEnv(t, defaultStorage, common.WithClock(clock))
	pluginWatcher := env.sut.NewResourceWatcher(t, types.KindPlugin)
	defer pluginWatcher.Close()

	const disallowedGroupName = "group3-disallowed"
	mustCreatePlugin(t,
		ctx,
		env.authClient,
		withSyncIntervals(&types.PluginEntraIDSyncIntervals{
			Delta: deltaSyncInterval.String(),
			Full:  "0", // full sync disabled
		}),
		withGroupFilters([]*types.PluginSyncFilter{
			{Exclude: &types.PluginSyncFilter_ExcludeNameRegex{ExcludeNameRegex: disallowedGroupName}},
		}),
	)

	// First full sync, should import all the user, group and group membership
	// defined in [newDefaultStorage].
	lastSyncTime := waitForPluginStatusUpdate(t, pluginWatcher, time.Time{} /* last sync time empty on first sync */)
	expectDefaultResources(t, env.authClient)

	gotAccesslists, err := listEntraIDAccessLists(ctx, env.authClient.AccessListClient())
	require.NoError(t, err, "list entra access lists")
	require.Len(t, gotAccesslists, 3) // Three groups defined in newDefaultStorage.

	// First delta sync.

	// Update group3 display so it matches with the exclude name filter.
	// This group should be eventually deleted.
	group3 := defaultStorage.Groups[group3ID]
	group3.DisplayName = to.Ptr(disallowedGroupName)

	aclWatcher := env.sut.NewResourceWatcher(t, types.KindAccessList)
	defer aclWatcher.Close()
	// Updating group3 also creates a delta diff for group3.
	env.fakeServer.SetGroups([]*models.Group{group3})
	clock.Advance(deltaSyncInterval)
	waitForPluginStatusUpdate(t, pluginWatcher, lastSyncTime)

	common.WaitForDeleteEvent(t, aclWatcher, func(r types.Resource) bool {
		return r.GetName() == gotAccesslists["group3"].GetName()
	})
	gotAccesslists, err = listEntraIDAccessLists(ctx, env.authClient.AccessListClient())
	require.NoError(t, err, "list entra access lists")
	require.Len(t, gotAccesslists, 2, "group3 not deleted")
}

func TestSynchronizationReset_DeltaEnabled_FullDisabled(t *testing.T) {
	ctx := t.Context()

	// Setup.
	defaultStorage := newDefaultStorage()
	clock := clockwork.NewFakeClock()
	env := newTestEnv(t, defaultStorage, common.WithClock(clock))
	pluginWatcher := env.sut.NewResourceWatcher(t, types.KindPlugin)
	defer pluginWatcher.Close()

	// Mock sync reset.
	//
	// Counter for how many times the "latest" delta API and the incremental
	// delta API was called. Each call to "latest" endpoint proves that
	// the sync was a full sync.
	var latestDeltaRequestCount, incrementalDeltaRequestCount atomic.Int64
	var resetErrorWritten atomic.Bool
	env.fakeServer.SetHandleListUsersDelta(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("$deltatoken")
		if token == "latest" {
			latestDeltaRequestCount.Add(1)
			env.fakeServer.ListUsersDelta(w, r)
			return
		}

		incrementalDeltaRequestCount.Add(1)
		if !resetErrorWritten.Swap(true) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusGone)
			const syncResetErr = `{
			"error": {
				"code": "resyncRequired",
				"message":"Exception of type resyncRequired was thrown.",
				"innerError": {
				"code": "resyncApplyDifferences",
				"request-id": "request-id",
				"date": "date-time"
				}
			}
			}`
			_, err := w.Write([]byte(syncResetErr))
			require.NoError(t, err)
			return
		}
		env.fakeServer.ListUsersDelta(w, r)
	})
	t.Cleanup(func() {
		env.fakeServer.SetHandleListUsersDelta(nil)
	})

	authClient := env.authClient
	mustCreatePlugin(t,
		ctx,
		authClient,
		withSyncIntervals(&types.PluginEntraIDSyncIntervals{
			Delta: deltaSyncInterval.String(),
			Full:  "0", // full sync disabled
		}),
	)

	// 1. First sync should be full even though the full interval is disabled.
	// Should import all users, groups, and group memberships defined in `newDefaultStorage`.

	// Wait for timer to be registered but do not advance the clock as the sync is supposed
	// to be scheduled instantly.
	advanceClock(t, ctx, clock, 0)
	lastSyncTime := waitForPluginStatusUpdate(t, pluginWatcher, time.Time{} /* last sync time empty on first sync */)
	expectDefaultUserSync(t, env.authClient)
	expectDefaultGroupSync(t, env.authClient)
	require.Equal(t, int64(1), latestDeltaRequestCount.Load(), "expected latest delta token request")
	require.Equal(t, int64(0), incrementalDeltaRequestCount.Load(), "first sync should not hit delta incremental API path")

	gotAccessLists, err := listEntraIDAccessLists(ctx, env.authClient.AccessListClient())
	require.NoError(t, err, "list entra access lists")
	require.Len(t, gotAccessLists, 3) // Three groups defined in newDefaultStorage.

	// 2. First delta sync after first full sync.
	// This sync schedule should hit the reset error.
	advanceClock(t, ctx, clock, deltaSyncInterval)
	lastSyncTime = waitForPluginStatusUpdate(t, pluginWatcher, lastSyncTime)
	// Latest API should not be called, but the incremental API should have been called.
	require.Equal(t, int64(1), latestDeltaRequestCount.Load(), "expected latest delta API to not be called")
	require.Equal(t, int64(1), incrementalDeltaRequestCount.Load(), "expected incremental API to have been called")

	// Existing resources should be intact.
	expectDefaultUserSync(t, env.authClient)
	expectDefaultGroupSync(t, env.authClient)

	// Previous delta sync was reset, the next sync should be a new full sync again.
	advanceClock(t, ctx, clock, entraid.DefaultFullSyncInterval)
	lastSyncTime = waitForPluginStatusUpdate(t, pluginWatcher, lastSyncTime, func(types.Plugin) bool {
		// expect latest delta count to be increased from 1 -> 2
		return latestDeltaRequestCount.Load() == 2 &&
			incrementalDeltaRequestCount.Load() == 1
	})

	// Existing resources should be intact.
	expectDefaultUserSync(t, env.authClient)
	expectDefaultGroupSync(t, env.authClient)

	// 3. Subsequent delta sync should return to normal incremental delta mode.
	// Add a new aliceN user and delete group1.
	env.fakeServer.DeleteGroups([]string{group1ID})
	aliceN := newEntraUser("848f8e0c-2714-4745-b478-1f7b59620ee5", "aliceN@example.com", "Alice N")
	env.fakeServer.SetUsers([]*models.User{aliceN})

	// Set watchers to wait for events.
	aclWatcher := env.sut.NewResourceWatcher(t, types.KindAccessList)
	defer aclWatcher.Close()
	userWatcher := env.sut.NewResourceWatcher(t, types.KindUser)
	defer userWatcher.Close()
	advanceClock(t, ctx, clock, deltaSyncInterval)
	waitForPluginStatusUpdate(t, pluginWatcher, lastSyncTime, func(types.Plugin) bool {
		// expect incremental delta count to be increased from 1 -> 2
		return latestDeltaRequestCount.Load() == 2 &&
			incrementalDeltaRequestCount.Load() == 2
	})
	common.WaitForDeleteEvent(t, aclWatcher, func(r types.Resource) bool {
		return r.GetName() == gotAccessLists["group1"].GetName()
	})

	// Group names matches with default payload available in [msgraphtest.PayloadListGroups].
	expectedAccessListTitles := []string{"group2", "group3"} // Group 1 was deleted.
	gotAccessLists, err = listEntraIDAccessLists(ctx, authClient.AccessListClient())
	require.NoError(t, err)
	require.NotNil(t, gotAccessLists)
	require.ElementsMatch(t, expectedAccessListTitles, slices.Collect(maps.Keys(gotAccessLists)), "expected access list to be deleted")

	common.WaitForPutEvent(t, userWatcher, func(r types.User) bool {
		return r.GetName() == "aliceN@example.com"
	})

	got, err := listEntraIDUsers(ctx, authClient)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{
		"alice@example.com",
		"bob@example.com",
		"carol@example.com",
		"aliceN@example.com",
	}, got, "expected Entra ID users to be created in Teleport")
}
