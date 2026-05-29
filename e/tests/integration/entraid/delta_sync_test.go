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
