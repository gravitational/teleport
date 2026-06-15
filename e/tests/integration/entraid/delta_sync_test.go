package entraid

import (
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
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
	expectDefaultPluginStatus(t, env.authClient, plugin.GetName())
	accessListClient := env.authClient.AccessListClient()
	expectDefaultUserSync(t, env.authClient)
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			// No membership that introduces cycle and all members are expected to be imported.

			gotAccesslists, err := listEntraIDAccessLists(ctx, accessListClient)
			require.NoError(t, err)
			require.ElementsMatch(t, []string{"group1", "group2", "group3"}, slices.Collect(maps.Keys(gotAccesslists)), "expected Entra ID groups to be created")

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
