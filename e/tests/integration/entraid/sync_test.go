package entraid

import (
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/lib/msgraph/models"
)

func TestResourceImport(t *testing.T) {
	ctx := t.Context()

	env := newTestEnv(t, newDefaultStorage())

	// Install the plugin, should trigger resource import on first start.
	plugin := newDefaultPluginSpec(t)
	err := createEntraIDPlugin(ctx, env.authClient, plugin)
	require.NoError(t, err, "expected Entra ID plugin to be created")

	expectDefaultPluginStatus(t, env.authClient, plugin.GetName())
	expectDefaultUserSync(t, env.authClient)
	expectDefaultGroupSync(t, env.authClient)
	expectDefaultGroupOwners(t, env.authClient)
}

func TestResourceImportWithUnsupportedUsers(t *testing.T) {
	ctx := t.Context()

	// Unsupported user with quote "'" in username.
	david := &models.User{
		DirectoryObject: models.DirectoryObject{
			ID:          to.Ptr("ed29ef7c-95ec-4754-aee4-683677a5d49c"),
			DisplayName: to.Ptr("David D"),
		},
		GivenName:         to.Ptr("David"),
		Surname:           to.Ptr("D"),
		Mail:              to.Ptr("dav'id@example.com"),
		UserPrincipalName: to.Ptr("dav'id@example.com"),
	}
	// Unsupported user with forward slash "/"" in username.
	eve := &models.User{
		DirectoryObject: models.DirectoryObject{
			ID:          to.Ptr("f889c1c3-c033-43ea-9a9b-8bfde82652f2"),
			DisplayName: to.Ptr("Eve E"),
		},
		GivenName:         to.Ptr("Eve"),
		Surname:           to.Ptr("E"),
		Mail:              to.Ptr("ev/e@example.com"),
		UserPrincipalName: to.Ptr("ev/e@example.com"),
	}
	// Supported username
	fiona := &models.User{
		DirectoryObject: models.DirectoryObject{
			ID:          to.Ptr("1205d54e-2f00-45f7-9e3a-ff7c176c4779"),
			DisplayName: to.Ptr("Fiona F"),
		},
		GivenName:         to.Ptr("Fiona"),
		Surname:           to.Ptr("F"),
		Mail:              to.Ptr("fiona@example.com"),
		UserPrincipalName: to.Ptr("fiona@example.com"),
	}

	defaultStorage := newDefaultStorage()
	// Add new users david, fiona, eve to the default storage users.
	defaultStorage.Users[*david.ID] = david
	defaultStorage.Users[*eve.ID] = eve
	defaultStorage.Users[*fiona.ID] = fiona
	// Add new group members david, fiona, eve to the default storage group members.
	defaultStorage.GroupMembers[group2ID] = slices.Concat(defaultStorage.GroupMembers[group2ID], []models.GroupMember{david, fiona, eve})
	defaultStorage.GroupMembers[group3ID] = slices.Concat(defaultStorage.GroupMembers[group3ID], []models.GroupMember{david, fiona, eve})

	env := newTestEnv(t, defaultStorage)

	// Install the plugin, should trigger resource import on first start.
	plugin := newDefaultPluginSpec(t)
	err := createEntraIDPlugin(ctx, env.authClient, plugin)
	require.NoError(t, err, "expected Entra ID plugin to be created")

	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			// alice, bob and carol are default users defined in newDefaultStorage().
			expected := []string{"alice@example.com", "bob@example.com", "carol@example.com", "fiona@example.com"} // david and eve users skipped.
			requireEntraIDUsers(t, ctx, env.authClient, expected)
		},
		time.Second*10, time.Millisecond*30)

	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			expectedAccessListTitles := []string{"group1", "group2", "group3"}
			gotAccesslists, err := listEntraIDAccessLists(ctx, env.authClient.AccessListClient())
			require.NoError(t, err)
			require.NotNil(t, gotAccesslists)
			require.ElementsMatch(t, expectedAccessListTitles, slices.Collect(maps.Keys(gotAccesslists)), "expected Entra ID groups to be created")

			// alice, bob and carol are default group members defined in newDefaultStorage().
			requireEntraIDAccessListMembers(t, ctx, env.authClient,
				gotAccesslists["group2"],
				[]string{"alice@example.com", "bob@example.com", "carol@example.com", "fiona@example.com"}, // david and eve users skipped.
			)
			requireEntraIDAccessListMembers(t, ctx, env.authClient,
				gotAccesslists["group3"],
				[]string{"alice@example.com", "bob@example.com", "carol@example.com", "fiona@example.com"}, // david and eve users skipped.
			)
		},
		time.Second*10, time.Millisecond*30)
}

func TestDeleteNestedAccessList(t *testing.T) {
	ctx := t.Context()

	storage := newDefaultStorage()
	// Add group1, group2 and group3.
	group1 := storage.Groups[group1ID]
	group2 := storage.Groups[group2ID]
	group3 := storage.Groups[group3ID]
	groups := make(map[string]*models.Group)
	groups[group1ID] = group1
	groups[group2ID] = group2
	groups[group3ID] = group3
	storage.Groups = groups

	// Add group2 and group3 as group1 members.
	groupMembership := make(map[string][]models.GroupMember)
	groupMembership[group1ID] = []models.GroupMember{group2, group3}
	groupMembership[group2ID] = []models.GroupMember{group3}
	storage.GroupMembers = groupMembership

	clock := clockwork.NewFakeClock()
	env := newTestEnv(t, storage, common.WithClock(clock))

	plugin := newDefaultPluginSpec(t)
	// Install the plugin, should trigger resource import on first start.
	err := createEntraIDPlugin(ctx, env.authClient, plugin)
	require.NoError(t, err, "expected Entra ID plugin to be created")

	expectPluginStatusUpdated(t, ctx, env.authClient, plugin.GetName(), clock)
	// default users alice, bob, carol that comes from newDefaultStorage.
	expectDefaultUserSync(t, env.authClient)
	// Wait for all Entra ID groups to synchronize to Teleport.
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			expectedAccessListTitles := []string{"group1", "group2", "group3"}
			gotAccesslists, err := listEntraIDAccessLists(ctx, env.authClient.AccessListClient())
			require.NoError(t, err)
			require.NotNil(t, gotAccesslists)
			require.ElementsMatch(t, expectedAccessListTitles, slices.Collect(maps.Keys(gotAccesslists)), "expected Entra ID groups to be created")

			requireEntraIDAccessListMembers(t, ctx, env.authClient,
				gotAccesslists["group1"],
				[]string{gotAccesslists["group2"].GetName(), gotAccesslists["group3"].GetName()},
			)
			requireEntraIDAccessListMembers(t, ctx, env.authClient,
				gotAccesslists["group2"],
				[]string{gotAccesslists["group3"].GetName()},
			)
		},
		time.Second*15, time.Millisecond*30)

	// Delete group2 and group3 from Entra ID.
	env.fakeServer.DeleteGroups([]string{group2ID, group3ID})
	expectPluginStatusUpdated(t, ctx, env.authClient, plugin.GetName(), clock)

	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			// group2 is deleted.
			expectedAccessListTitles := []string{"group1"}
			gotAccesslists, err := listEntraIDAccessLists(ctx, env.authClient.AccessListClient())
			require.NoError(t, err)
			require.NotNil(t, gotAccesslists)
			require.ElementsMatch(t, expectedAccessListTitles, slices.Collect(maps.Keys(gotAccesslists)), "expected Entra ID group to be deleted")

			// group2 membership is removed.
			gotMembers, err := listEntraIDMembers(ctx, gotAccesslists["group1"].GetName(), env.authClient.AccessListClient())
			require.NoError(t, err, "listing Access List members for Entra ID groups")
			require.Empty(t, gotMembers)

		},
		time.Second*15, time.Millisecond*30)
}

func TestResourceImportWithCyclicGroupMembers(t *testing.T) {
	ctx := t.Context()
	defaultStorage := newDefaultStorage()
	alice := defaultStorage.Users[aliceID]
	bob := defaultStorage.Users[bobID]
	group1 := defaultStorage.Groups[group1ID]
	group2 := defaultStorage.Groups[group2ID]
	group3 := defaultStorage.Groups[group3ID]

	defaultStorage.GroupMembers = make(map[string][]models.GroupMember)
	defaultStorage.GroupMembers[*group1.GetID()] = []models.GroupMember{group2}
	// Create cyclic membership between group1 and group2
	defaultStorage.GroupMembers[*group2.GetID()] = []models.GroupMember{group1}
	defaultStorage.GroupMembers[*group3.GetID()] = []models.GroupMember{group1, alice, bob}

	clock := clockwork.NewFakeClock()
	env := newTestEnv(t, defaultStorage, common.WithClock(clock))

	// Install the plugin, should trigger resource import on first start.
	plugin := newDefaultPluginSpec(t)
	settings := plugin.Spec.GetEntraId().SyncSettings
	settings.SyncIntervals = &types.PluginEntraIDSyncIntervals{
		Full: "5m",
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
	expectFailedPluginStatus := func(t *testing.T) {
		t.Helper()

		require.EventuallyWithT(t,
			func(t *assert.CollectT) {
				updatedPlugin, err := env.authClient.PluginsClient().GetPlugin(ctx, pluginsv1.GetPluginRequest_builder{
					Name: plugin.GetName(),
				}.Build())
				require.NoError(t, err)

				status := updatedPlugin.GetStatus()
				require.Contains(t, status.GetLastRawError(), "is already included as a Member or Owner")
				require.Equal(t, types.PluginStatusCode_OTHER_ERROR, status.GetCode(), `expected plugin status to be "OTHER_ERROR"`)

				entraStatus := status.GetEntraId()
				require.NotNil(t, entraStatus)
				// No changes in expected user or group import count.
				require.Equal(t, uint32(3), entraStatus.ImportedUsers)
				require.Equal(t, uint32(3), entraStatus.ImportedGroups)
			},
			time.Second*15, time.Millisecond*30, "plugin status")
	}
	expectFailedPluginStatus(t)
	expectDefaultUserSync(t, env.authClient)

	accessListClient := env.authClient.AccessListClient()
	expectFilteredMembers := func(t *testing.T) {
		t.Helper()

		require.EventuallyWithT(t,
			func(t *assert.CollectT) {
				gotAccesslists, err := listEntraIDAccessLists(ctx, accessListClient)
				require.NoError(t, err)
				expectedDefaultAccessListTitles := []string{"group1", "group2", "group3"}
				require.ElementsMatch(t, expectedDefaultAccessListTitles, slices.Collect(maps.Keys(gotAccesslists)), "expected Entra ID groups to be created")

				// No changes expected in group1.
				group1members := mustListMembers(t, ctx, gotAccesslists["group1"].GetName(), accessListClient)
				require.Len(t, group1members, 1)

				// Cyclic membership removal is of sorted order. group1 -> group2 is evaluated first.
				// Then group2 -> group1 which introduces cycle and is removed.
				group2Members := mustListMembers(t, ctx, gotAccesslists["group2"].GetName(), accessListClient)
				require.Empty(t, group2Members)

				// No changes expected in group3.
				group3Members := mustListMembers(t, ctx, gotAccesslists["group3"].GetName(), accessListClient)
				require.ElementsMatch(t, []string{gotAccesslists["group1"].GetName(), "alice@example.com", "bob@example.com"}, group3Members, "expected Entra ID group members to match")
			},
			time.Second*20, time.Millisecond*30)
	}
	expectFilteredMembers(t)

	// Update plugin to trigger re-sync.
	// This time, the sync will skip bulk collection insert and will move to reconciler.
	pluginToUpdate, err := env.authClient.PluginsClient().GetPlugin(ctx, pluginsv1.GetPluginRequest_builder{
		Name: plugin.GetName(),
	}.Build())
	require.NoError(t, err)
	settings = pluginToUpdate.Spec.GetEntraId().SyncSettings
	settings.SyncIntervals.Full = "2h"
	plugin.Spec.Settings = &types.PluginSpecV1_EntraId{
		EntraId: &types.PluginEntraIDSettings{
			SyncSettings: settings,
		},
	}
	err = updateEntraIDPlugin(ctx, env.authClient, pluginToUpdate)
	require.NoError(t, err, "expected Entra ID plugin to be updated")

	// In the first sync, resources are bulk inserted. In subsequent full sync
	// resources are upserted using reconciler. But the resource assertion
	// must be the same between those two syncs.
	expectFailedPluginStatus(t)
	expectDefaultUserSync(t, env.authClient)
	expectFilteredMembers(t)
}

func TestFullSyncReImportsDirectory(t *testing.T) {
	ctx := t.Context()

	// Setup.
	clock := clockwork.NewFakeClock()
	env := newTestEnv(t, newDefaultStorage(), common.WithClock(clock))
	pluginWatcher := env.sut.NewResourceWatcher(t, types.KindPlugin)
	defer pluginWatcher.Close()
	userWatcher := env.sut.NewResourceWatcher(t, types.KindUser)
	defer userWatcher.Close()
	aclWatcher := env.sut.NewResourceWatcher(t, types.KindAccessList)
	defer aclWatcher.Close()

	const fullSyncInterval = 15 * time.Second
	mustCreatePlugin(t,
		ctx,
		env.authClient,
		withSyncIntervals(&types.PluginEntraIDSyncIntervals{
			Delta: "0", // delta sync disabled
			Full:  fullSyncInterval.String(),
		}),
		withAclOwnersConfig(types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_ENTRAID),
	)

	// First full sync.
	lastSyncTime := waitForPluginStatusUpdate(t, pluginWatcher, time.Time{} /* last sync time empty on first sync */)
	expectDefaultResourcesWithEntraOwners(t, ctx, env.authClient)

	// Delete user resources alice, bob and carol created from default storage.
	const userToDelete = "bob@example.com"
	mustDeleteUsersFromTeleport(t, env.authClient, []string{userToDelete})
	common.WaitForDeleteEvent(t, userWatcher, func(r types.Resource) bool {
		return r.GetName() == userToDelete
	})
	// Default groups - group1, group2, group3.
	gotAccesslists, err := listEntraIDAccessLists(ctx, env.authClient.AccessListClient())
	require.NoError(t, err)
	aclToDelete := gotAccesslists["group3"].GetName()
	mustDeleteAccessLists(t, env.authClient, []string{aclToDelete})
	common.WaitForDeleteEvent(t, aclWatcher, func(r types.Resource) bool {
		return r.GetName() == aclToDelete
	})

	// Wait for another sync.
	clock.Advance(fullSyncInterval)
	waitForPluginStatusUpdate(t, pluginWatcher, lastSyncTime)

	// Expect all the same resources checked after first full sync.
	expectDefaultResourcesWithEntraOwners(t, ctx, env.authClient)
}
