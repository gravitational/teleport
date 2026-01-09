package entraid

import (
	"context"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/msgraph"
	"github.com/gravitational/teleport/lib/msgraph/msgraphtest"
)

func TestResourceImport(t *testing.T) {
	ctx := t.Context()

	fakeServer := msgraphtest.NewServer(
		msgraphtest.WithStorage(msgraphtest.NewDefaultStorage()),
	)
	t.Cleanup(func() { fakeServer.TLSServer.Close() })

	httpClient := &http.Client{
		Transport: &msgraphtest.RewriteTransport{
			Base: fakeServer.TLSServer.Client().Transport,
			URL:  mustParseURL(t, fakeServer.TLSServer.URL),
		},
	}

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.EntraIDSAMLConnector(connectorName)),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "user-editor", "editor"),
		common.WithHTTPClient(httpClient.Transport),
	)

	editorClient := sut.GetClusterClientForUser(t, "user-editor").AuthClient

	// Install the plugin, should trigger resource import on first start.
	err := createEntraIDPlugin(ctx, editorClient)
	require.NoError(t, err, "expected Entra ID plugin to be created")

	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			// Users are expected to be created before Access List and members.
			// User names matches with default payload available in [msgraphtest.PayloadListUsers].
			expected := []string{"alice@example.com", "bob@example.com", "carol@example.com"}
			requireEntraIDUsers(t, ctx, editorClient, expected)
		},
		time.Second*10, time.Millisecond*30)

	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			// Entra ID groups are created as Access List and
			// group members created as Access List members.

			// Group names matches with default payload available in [msgraphtest.PayloadListGroups].
			expectedAccessListTitles := []string{"group1", "group2", "group3"}
			gotAccesslists, err := listEntraIDAccessLists(ctx, editorClient.AccessListClient())
			require.NoError(t, err)
			require.NotNil(t, gotAccesslists)
			require.ElementsMatch(t, expectedAccessListTitles, slices.Collect(maps.Keys(gotAccesslists)), "expected Entra ID groups to be created")

			requireEntraIDAccessListMembers(t, ctx, editorClient,
				gotAccesslists["group1"].GetName(),
				[]string{gotAccesslists["group2"].GetName(), "alice@example.com"}, // group2 is nested member of group1.
			)

			requireEntraIDAccessListMembers(t, ctx, editorClient,
				gotAccesslists["group2"].GetName(),
				[]string{"alice@example.com", "bob@example.com", "carol@example.com"},
			)

			requireEntraIDAccessListMembers(t, ctx, editorClient,
				gotAccesslists["group3"].GetName(),
				[]string{"alice@example.com", "bob@example.com", "carol@example.com"},
			)
		},
		time.Second*10, time.Millisecond*30)
}

func TestResourceImportWithUnsupportedUsers(t *testing.T) {
	ctx := t.Context()

	// Unsupported user with quote "'" in username.
	david := &msgraph.User{
		DirectoryObject: msgraph.DirectoryObject{
			ID:          to.Ptr("dav'id@example.com"),
			DisplayName: to.Ptr("David D"),
		},
		GivenName:         to.Ptr("David"),
		Surname:           to.Ptr("D"),
		Mail:              to.Ptr("dav'id@example.com"),
		UserPrincipalName: to.Ptr("dav'id@example.com"),
	}
	// Unsupported user with forward slash "/"" in username.
	eve := &msgraph.User{
		DirectoryObject: msgraph.DirectoryObject{
			ID:          to.Ptr("ev/e@example.com"),
			DisplayName: to.Ptr("Eve E"),
		},
		GivenName:         to.Ptr("Eve"),
		Surname:           to.Ptr("E"),
		Mail:              to.Ptr("ev/e@example.com"),
		UserPrincipalName: to.Ptr("ev/e@example.com"),
	}
	// Supported username
	fiona := &msgraph.User{
		DirectoryObject: msgraph.DirectoryObject{
			ID:          to.Ptr("fiona@example.com"),
			DisplayName: to.Ptr("Fiona F"),
		},
		GivenName:         to.Ptr("Fiona"),
		Surname:           to.Ptr("F"),
		Mail:              to.Ptr("fiona@example.com"),
		UserPrincipalName: to.Ptr("fiona@example.com"),
	}

	defaultStorage := msgraphtest.NewDefaultStorage()
	// Add new users david, fiona, eve to the default storage users.
	defaultStorage.Users[*david.ID] = david
	defaultStorage.Users[*eve.ID] = eve
	defaultStorage.Users[*fiona.ID] = fiona
	// Add new group members david, fiona, eve to the default storage group members.
	defaultStorage.GroupMembers["group2"] = slices.Concat(defaultStorage.GroupMembers["group2"], []msgraph.GroupMember{david, fiona, eve})
	defaultStorage.GroupMembers["group3"] = slices.Concat(defaultStorage.GroupMembers["group3"], []msgraph.GroupMember{david, fiona, eve})

	// Setup test env
	fakeServer := msgraphtest.NewServer(
		msgraphtest.WithStorage(defaultStorage),
	)
	t.Cleanup(func() { fakeServer.TLSServer.Close() })

	httpClient := &http.Client{
		Transport: &msgraphtest.RewriteTransport{
			Base: fakeServer.TLSServer.Client().Transport,
			URL:  mustParseURL(t, fakeServer.TLSServer.URL),
		},
	}

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.EntraIDSAMLConnector(connectorName)),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "user-editor", "editor"),
		common.WithHTTPClient(httpClient.Transport),
	)

	authClient := sut.GetClusterClientForUser(t, "user-editor").AuthClient

	// Install the plugin, should trigger resource import on first start.
	err := createEntraIDPlugin(ctx, authClient)
	require.NoError(t, err, "expected Entra ID plugin to be created")

	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			// alice, bob and carol are default users defined in
			// [msgraphtest.PayloadListUsers].
			expected := []string{"alice@example.com", "bob@example.com", "carol@example.com", "fiona@example.com"} // david and eve users skipped.
			requireEntraIDUsers(t, ctx, authClient, expected)
		},
		time.Second*10, time.Millisecond*30)

	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			expectedAccessListTitles := []string{"group1", "group2", "group3"}
			gotAccesslists, err := listEntraIDAccessLists(ctx, authClient.AccessListClient())
			require.NoError(t, err)
			require.NotNil(t, gotAccesslists)
			require.ElementsMatch(t, expectedAccessListTitles, slices.Collect(maps.Keys(gotAccesslists)), "expected Entra ID groups to be created")

			// alice, bob and carol are default group members defined
			// in [msgraphtest.PayloadListGroups].
			requireEntraIDAccessListMembers(t, ctx, authClient,
				gotAccesslists["group2"].GetName(),
				[]string{"alice@example.com", "bob@example.com", "carol@example.com", "fiona@example.com"}, // david and eve users skipped.
			)
			requireEntraIDAccessListMembers(t, ctx, authClient,
				gotAccesslists["group3"].GetName(),
				[]string{"alice@example.com", "bob@example.com", "carol@example.com", "fiona@example.com"}, // david and eve users skipped.
			)
		},
		time.Second*10, time.Millisecond*30)
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

func requireEntraIDAccessListMembers(t *assert.CollectT, ctx context.Context, authClient authclient.ClientI, aclName string, expectedMembers []string) {
	t.Helper()

	gotMembers, err := listEntraIDMembers(ctx, aclName, authClient.AccessListClient())
	require.NoError(t, err, "listing Access List members for Entra ID groups")
	require.NotNil(t, gotMembers)

	require.ElementsMatch(t, expectedMembers, gotMembers, "expected Entra ID group members to be created")
}
