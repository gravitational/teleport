package entraid

import (
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/msgraph/models"
)

func TestSyncErrorStatusReport(t *testing.T) {
	ctx := t.Context()

	storage := newDefaultStorage()
	// Unsupported user with quote "'" in username.
	davidInvalid := newEntraUser("f25cfcf2-913d-4fd0-a2d3-4d7e7ac2100b", "dav'id@example.com", "David I")
	// Unsupported user with forward slash "/"" in username.
	eveInvalid := newEntraUser("5cba2a74-f905-4a6c-a9d5-d43a3082bed8", "ev/e@example.com", "Eve E")
	// Supported username.
	fiona := newEntraUser("adc68501-3dc8-4ab9-a1f3-a2389928a840", "fiona@example.com", "Fiona F")
	// Supported username but a local user with same email already exists.
	gianna := newEntraUser("899d9091-7bbf-4e7c-88be-9b044f36d36b", "gianna@example.com", "Gianna G")
	// Add new users david, fiona, eve to the default storage users,
	// Default users alice, bob and carol remains unchanged.
	storage.Users[*davidInvalid.ID] = davidInvalid
	storage.Users[*eveInvalid.ID] = eveInvalid
	storage.Users[*fiona.ID] = fiona
	storage.Users[*gianna.ID] = gianna

	group1 := newEntraGroup(group1ID, "group1")
	group2 := newEntraGroup(group2ID, "group2")
	group3invalid := &models.Group{
		DirectoryObject: models.DirectoryObject{
			ID:          to.Ptr("group3invalid"),
			DisplayName: nil, // trigger group conversion error, causing this group to be skipped.
		},
	}
	storage.Groups = make(map[string]*models.Group)
	storage.Groups[group1ID] = group1
	storage.Groups[group2ID] = group2
	storage.Groups["group3invalid"] = group3invalid

	// Add new group members david, fiona, eve to the default storage group members.
	storage.GroupMembers[group1ID] = slices.Concat(storage.GroupMembers[group1ID], []models.GroupMember{davidInvalid, fiona, eveInvalid})
	storage.GroupMembers[group2ID] = slices.Concat(storage.GroupMembers[group2ID], []models.GroupMember{davidInvalid, fiona, eveInvalid})

	env := newTestEnv(t, storage)

	// Create local user account "gianna" to simulate duplicate user.
	user, err := types.NewUser("gianna@example.com")
	require.NoError(t, err)
	_, err = env.authClient.CreateUser(ctx, user)
	require.NoError(t, err, "expected local user account to be created")

	// Install the plugin, should trigger resource import on first start.
	plugin := newDefaultPluginSpec(t)
	err = createEntraIDPlugin(ctx, env.authClient, plugin)
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
			expectedAccessListTitles := []string{"group1", "group2"}
			gotAccesslists, err := listEntraIDAccessLists(ctx, env.authClient.AccessListClient())
			require.NoError(t, err)
			require.NotNil(t, gotAccesslists)
			require.ElementsMatch(t, expectedAccessListTitles, slices.Collect(maps.Keys(gotAccesslists)), "expected Entra ID groups to be created")

			// alice, bob and carol are default group members defined in newDefaultStorage().
			requireEntraIDAccessListMembers(t, ctx, env.authClient,
				gotAccesslists["group1"],
				[]string{"alice@example.com", gotAccesslists["group2"].GetName(), "fiona@example.com"}, // david and eve users skipped.
			)
			requireEntraIDAccessListMembers(t, ctx, env.authClient,
				gotAccesslists["group2"],
				[]string{"alice@example.com", "bob@example.com", "carol@example.com", "fiona@example.com"}, // david and eve users skipped.
			)
		},
		time.Second*10, time.Millisecond*30)

	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			updatedPlugin, err := env.authClient.PluginsClient().GetPlugin(ctx, pluginsv1.GetPluginRequest_builder{
				Name: plugin.GetName(),
			}.Build())
			require.NoError(t, err)

			status := updatedPlugin.GetStatus()
			require.NotEqual(t, types.PluginStatusCode_RUNNING, status.GetCode())
			require.Contains(t, status.GetErrorMessage(), "completed with partial success")
			require.Contains(t, status.GetLastRawError(), "username contains unsupported character(s)")
			require.Contains(t, status.GetLastRawError(), "ev/e@example.com")
			require.Contains(t, status.GetLastRawError(), "dav'id@example.com")
			require.Contains(t, status.GetLastRawError(), "existing user account found")
			require.Contains(t, status.GetLastRawError(), "gianna@example.com")

			require.Contains(t, status.GetLastRawError(), "expected Entra ID group(id=group3invalid) to have a non-empty display name")

			entraStatus := status.GetEntraId()
			require.Equal(t, uint32(4), entraStatus.ImportedUsers)
			require.Equal(t, uint32(2), entraStatus.ImportedGroups)
		},
		time.Second*10, time.Millisecond*30, "expected skipped users and groups to be reported")
}
