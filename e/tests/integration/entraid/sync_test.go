package entraid

import (
	"context"
	"maps"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/auth/authclient"
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

func expectDefaultUserSync(t *testing.T, authClt authclient.ClientI) {
	t.Helper()

	ctx := t.Context()
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			// Users are expected to be created before Access List and members.
			// User names matches with default payload available in newDefaultStorage().
			expected := []string{"alice@example.com", "bob@example.com", "carol@example.com"}
			requireEntraIDUsers(t, ctx, authClt, expected)
		},
		time.Second*10, time.Millisecond*30)
}

func expectDefaultGroupSync(t *testing.T, authClt authclient.ClientI) {
	t.Helper()

	ctx := t.Context()
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			// Entra ID groups are created as Access List and
			// group members created as Access List members.

			// Group names matches with default payload available in newDefaultStorage().
			expectedAccessListTitles := []string{"group1", "group2", "group3"}
			gotAccesslists, err := listEntraIDAccessLists(ctx, authClt.AccessListClient())
			require.NoError(t, err)
			require.NotNil(t, gotAccesslists)
			require.ElementsMatch(t, expectedAccessListTitles, slices.Collect(maps.Keys(gotAccesslists)), "expected Entra ID groups to be created")

			requireEntraIDAccessListMembers(t, ctx, authClt,
				gotAccesslists["group1"],
				[]string{gotAccesslists["group2"].GetName(), "alice@example.com"}, // group2 is nested member of group1.
			)

			requireEntraIDAccessListMembers(t, ctx, authClt,
				gotAccesslists["group2"],
				[]string{"alice@example.com", "bob@example.com", "carol@example.com"},
			)

			requireEntraIDAccessListMembers(t, ctx, authClt,
				gotAccesslists["group3"],
				[]string{"alice@example.com", "bob@example.com", "carol@example.com"},
			)

		},
		time.Second*10, time.Millisecond*30)
}

func expectDefaultGroupOwners(t *testing.T, authClt authclient.ClientI) {
	t.Helper()

	ctx := t.Context()
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			gotAccesslists, err := listEntraIDAccessLists(ctx, authClt.AccessListClient())
			require.NoError(t, err)
			require.NotNil(t, gotAccesslists)
			requireDefaultEntraIDAccessListOwners(t, gotAccesslists)
		},
		time.Second*10, time.Millisecond*30)
}

func expectDefaultPluginStatus(t *testing.T, authClt authclient.ClientI, name string) {
	t.Helper()

	ctx := t.Context()
	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			updatedPlugin, err := authClt.PluginsClient().GetPlugin(ctx, &pluginsv1.GetPluginRequest{
				Name: name,
			})
			require.NoError(t, err)

			status := updatedPlugin.GetStatus()
			require.Empty(t, status.GetLastRawError())
			require.Empty(t, status.GetErrorMessage())
			require.Equal(t, types.PluginStatusCode_RUNNING, status.GetCode(), `expected plugin status to be "running"`)

			entraStatus := status.GetEntraId()
			require.NotNil(t, entraStatus)
			require.Equal(t, uint32(3), entraStatus.ImportedUsers)
			require.Equal(t, uint32(3), entraStatus.ImportedGroups)
		},
		time.Second*15, time.Millisecond*30, "expected successful plugins status")
}

func mustParseURL(t *testing.T, in string) *url.URL {
	t.Helper()
	url, err := url.Parse(in)
	require.NoError(t, err)
	require.Equal(t, "https", url.Scheme, "expected URL with https scheme")
	return url
}

func requireEntraIDUsers(t *assert.CollectT, ctx context.Context, authClient authclient.ClientI, expectedUsers []string) {
	t.Helper()

	got, err := listEntraIDUsers(ctx, authClient)
	require.NoError(t, err)
	require.NotNil(t, got)

	require.ElementsMatch(t, expectedUsers, got, "expected Entra ID users to be created in Teleport")
}

func requireEntraIDAccessListMembers(t *assert.CollectT, ctx context.Context, authClient authclient.ClientI, acl *accesslist.AccessList, expectedMembers []string) {
	t.Helper()

	gotMembers, err := listEntraIDMembers(ctx, acl.GetName(), authClient.AccessListClient())
	require.NoError(t, err, "listing Access List members for Entra ID groups")
	require.NotNil(t, gotMembers)

	require.ElementsMatch(t, expectedMembers, gotMembers, "expected Entra ID group members to be created. acl=%s", acl.Spec.Title)
}

func requireDefaultEntraIDAccessListOwners(t *assert.CollectT, acls map[string]*accesslist.AccessList) {
	t.Helper()

	for _, acl := range acls {
		require.Empty(t,
			cmp.Diff(acl.Spec.Owners, []accesslist.Owner{defaultOwner},
				cmpopts.IgnoreFields(accesslist.Owner{}, "IneligibleStatus"),
			), "expected Entra ID group owners to match")
	}
}

func requireRoleAndTraits(t *testing.T, ctx context.Context, authClt authclient.ClientI, username string, expectedRoles []string, expectedTraits []string, msg string) {
	t.Helper()

	user, err := authClt.GetUser(ctx, username, false)
	require.NoError(t, err)
	require.ElementsMatch(t, expectedRoles, user.GetRoles(), msg)
	userGroupTraits := user.GetTraits()["http://schemas.microsoft.com/ws/2008/06/identity/claims/groups"]
	require.ElementsMatch(t, expectedTraits, userGroupTraits, msg)
}
