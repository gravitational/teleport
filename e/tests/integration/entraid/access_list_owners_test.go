package entraid

import (
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/e/lib/entraid"
	"github.com/gravitational/teleport/lib/msgraph"
	"github.com/gravitational/teleport/lib/msgraph/msgraphtest"
)

// Test that when AccessListOwnersSource is set to
// EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_PLUGIN,
// Entra ID group owners should be ignored and Access List should be configured with
// default owners.
func TestAccessListDefaultOwners(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	defaultStorage := msgraphtest.NewDefaultStorage()
	env := newTestEnv(t, defaultStorage)

	// Plugin configured with plugin as a source of Access List owners.
	plugin := newDefaultPluginSpec(t)
	settings := plugin.Spec.GetEntraId().SyncSettings
	settings.AccessListOwnersSource = types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_PLUGIN
	plugin.Spec.Settings = &types.PluginSpecV1_EntraId{
		EntraId: &types.PluginEntraIDSettings{
			SyncSettings: settings,
		},
	}

	// Install the plugin, should trigger resource import on first start.
	err := createEntraIDPlugin(ctx, env.authClient, plugin)
	require.NoError(t, err, "expected Entra ID plugin to be created")

	// Default assertion of user and groups.
	expectDefaultUserSync(t, env.authClient)
	expectDefaultGroupSync(t, env.authClient)
	expectDefaultGroupOwners(t, env.authClient)
}

// Test that when AccessListOwnersSource is set to
// EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_ENTRAID,
// Entra ID group owners should be used as Access List owners.
func TestAccessListEntraIDGroupOwners(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	defaultStorage := msgraphtest.NewDefaultStorage()
	env := newTestEnv(t, defaultStorage)

	// Plugin with Entra ID group owners as Access List owner source.
	plugin := newDefaultPluginSpec(t)
	settings := plugin.Spec.GetEntraId().SyncSettings
	settings.AccessListOwnersSource = types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_ENTRAID
	plugin.Spec.Settings = &types.PluginSpecV1_EntraId{
		EntraId: &types.PluginEntraIDSettings{
			SyncSettings: settings,
		},
	}
	// Install the plugin, should trigger resource import on first start.
	err := createEntraIDPlugin(ctx, env.authClient, plugin)
	require.NoError(t, err, "expected Entra ID plugin to be created")

	// Default assertion of user and groups.
	expectDefaultUserSync(t, env.authClient)
	expectDefaultGroupSync(t, env.authClient)

	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			gotAccesslists, err := listEntraIDAccessLists(ctx, env.authClient.AccessListClient())
			require.NoError(t, err, "listing entra id access lists")

			expectedGroup1Owners := entraid.ToAclOwner(ctx, defaultStorage.GroupOwners["group1"])
			require.Empty(t,
				cmp.Diff(expectedGroup1Owners, gotAccesslists["group1"].Spec.Owners,
					cmpopts.IgnoreFields(accesslist.Owner{}, "IneligibleStatus"),
				), "expected Entra ID group1 owners to match")

			// group2 has zero owners, should fallback to default owners.
			require.Empty(t,
				cmp.Diff([]accesslist.Owner{defaultOwner}, gotAccesslists["group2"].Spec.Owners,
					cmpopts.IgnoreFields(accesslist.Owner{}, "IneligibleStatus"),
				), "expected Entra ID group2 owners to match")

			expectedGroup3Owners := entraid.ToAclOwner(ctx, defaultStorage.GroupOwners["group3"])
			require.Empty(t,
				cmp.Diff(expectedGroup3Owners, gotAccesslists["group3"].Spec.Owners,
					cmpopts.IgnoreFields(accesslist.Owner{}, "IneligibleStatus"),
				), "expected Entra ID group3 owners to match")

		},
		time.Second*10, time.Millisecond*30)
}

// Test that when AccessListOwnersSource is set to
// EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_ENTRAID,
// but Entra ID groups have unsupported user account. Service should filter
// unsupported user. If all the group owners are filtered, service should fall
// back to using default owners.
func TestAccessListUnsupportedEntraIDGroupOwners(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	defaultStorage := msgraphtest.NewDefaultStorage()
	env := newTestEnv(t, defaultStorage)

	// fion'a@example.com is an unsupported user account (contains quote "'"),
	// should be filtered if configured as group owner.
	fiona := &msgraph.User{
		DirectoryObject: msgraph.DirectoryObject{
			ID:          to.Ptr("fion'a@example.com"),
			DisplayName: to.Ptr("Fiona F"),
		},
		GivenName:         to.Ptr("Fiona"),
		Surname:           to.Ptr("F"),
		Mail:              to.Ptr("fion'a@example.com"),
		UserPrincipalName: to.Ptr("fion'a@example.com"),
	}
	defaultStorage.Users[*fiona.ID] = fiona
	users := slices.Concat(slices.Collect(maps.Values(defaultStorage.Users)))
	users = append(users, fiona)
	env.fakeServer.SetUsers(users)

	// Append one unsupported user account to group1 owner.
	group1Owners := defaultStorage.GroupOwners["group1"]
	group1Owners = append(group1Owners, fiona)
	env.fakeServer.SetGroupOwners("group1", group1Owners)
	// Add one unsupported user account as group3 owner.
	env.fakeServer.SetGroupOwners("group3", []*msgraph.User{fiona})

	// Plugin with Entra ID group owners as Access List owner source.
	plugin := newDefaultPluginSpec(t)
	settings := plugin.Spec.GetEntraId().SyncSettings
	settings.AccessListOwnersSource = types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_ENTRAID
	plugin.Spec.Settings = &types.PluginSpecV1_EntraId{
		EntraId: &types.PluginEntraIDSettings{
			SyncSettings: settings,
		},
	}
	// Install the plugin, should trigger resource import on first start.
	err := createEntraIDPlugin(ctx, env.authClient, plugin)
	require.NoError(t, err, "expected Entra ID plugin to be created")

	// Default assertion of user and groups.
	expectDefaultUserSync(t, env.authClient)
	expectDefaultGroupSync(t, env.authClient)

	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			gotAccesslists, err := listEntraIDAccessLists(ctx, env.authClient.AccessListClient())
			require.NoError(t, err, "listing entra id access lists")

			expectedGroup1Owners := entraid.ToAclOwner(ctx, group1Owners)
			require.Len(t, expectedGroup1Owners, 2)
			require.Empty(t,
				cmp.Diff(expectedGroup1Owners, gotAccesslists["group1"].Spec.Owners,
					cmpopts.IgnoreFields(accesslist.Owner{}, "IneligibleStatus"),
				), "expected Entra ID group1 owners to match")

			// group2 has zero owners, should fallback to default owners.
			require.Empty(t,
				cmp.Diff([]accesslist.Owner{defaultOwner}, gotAccesslists["group2"].Spec.Owners,
					cmpopts.IgnoreFields(accesslist.Owner{}, "IneligibleStatus"),
				), "expected Entra ID group2 owners to match")

			// group3 had only one owner but the user account is unsupported and filtered.
			// Should fallback to default owners.
			require.Empty(t,
				cmp.Diff([]accesslist.Owner{defaultOwner}, gotAccesslists["group3"].Spec.Owners,
					cmpopts.IgnoreFields(accesslist.Owner{}, "IneligibleStatus"),
				), "expected Entra ID group3 owners to match")

		},
		time.Second*10, time.Millisecond*30)
}

// Test that when AccessListOwnersSource is set to
// EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_PLUGIN_AND_ENTRAID,
// service should merge both Entra ID group owners and plugin default owners.
func TestAccessListMergePluginAndEntraIDGroupOwners(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	defaultStorage := msgraphtest.NewDefaultStorage()
	env := newTestEnv(t, defaultStorage)

	// Plugin with Entra ID group owners and Plugin source as Access List owners.
	plugin := newDefaultPluginSpec(t)
	settings := plugin.Spec.GetEntraId().SyncSettings
	settings.AccessListOwnersSource = types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_PLUGIN_AND_ENTRAID
	plugin.Spec.Settings = &types.PluginSpecV1_EntraId{
		EntraId: &types.PluginEntraIDSettings{
			SyncSettings: settings,
		},
	}
	// Install the plugin, should trigger resource import on first start.
	err := createEntraIDPlugin(ctx, env.authClient, plugin)
	require.NoError(t, err, "expected Entra ID plugin to be created")

	// Default assertion of user and groups.
	expectDefaultUserSync(t, env.authClient)
	expectDefaultGroupSync(t, env.authClient)

	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			gotAccesslists, err := listEntraIDAccessLists(ctx, env.authClient.AccessListClient())
			require.NoError(t, err, "listing entra id access lists")

			defaultOwners := []accesslist.Owner{defaultOwner}
			// Merge Entra ID group owners and plugin default owners.
			expectedGroup1Owners := slices.Concat(entraid.ToAclOwner(ctx, defaultStorage.GroupOwners["group1"]), defaultOwners)
			require.Empty(t,
				cmp.Diff(expectedGroup1Owners, gotAccesslists["group1"].Spec.Owners,
					cmpopts.IgnoreFields(accesslist.Owner{}, "IneligibleStatus"),
				), "expected Entra ID group1 owners to match")

			// group2 has zero owners, should fallback to default owners.
			require.Empty(t,
				cmp.Diff(defaultOwners, gotAccesslists["group2"].Spec.Owners,
					cmpopts.IgnoreFields(accesslist.Owner{}, "IneligibleStatus"),
				), "expected Entra ID group2 owners to match")

			// Merge Entra ID group owners and plugin default owners.
			expectedGroup3Owners := slices.Concat(entraid.ToAclOwner(ctx, defaultStorage.GroupOwners["group3"]), defaultOwners)
			require.Empty(t,
				cmp.Diff(expectedGroup3Owners, gotAccesslists["group3"].Spec.Owners,
					cmpopts.IgnoreFields(accesslist.Owner{}, "IneligibleStatus"),
				), "expected Entra ID group3 owners to match")

		},
		time.Second*10, time.Millisecond*30)
}
