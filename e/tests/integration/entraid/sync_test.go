package entraid

import (
	"context"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/lib/auth/authclient"
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
			expectedGroups := []string{"group1", "group2", "group3"}
			gotGroups, err := listEntraIDAccessLists(ctx, editorClient.AccessListClient())
			require.NoError(t, err)
			require.NotNil(t, gotGroups)
			require.ElementsMatch(t, expectedGroups, slices.Collect(maps.Keys(gotGroups)), "expected Entra ID groups to be created")

			requireEntraIDAccessListMembers(t, ctx, editorClient,
				gotGroups["group1"],
				[]string{gotGroups["group2"], "alice@example.com"}, // group2 is nested member of group1.
			)

			requireEntraIDAccessListMembers(t, ctx, editorClient,
				gotGroups["group2"],
				[]string{"alice@example.com", "bob@example.com", "carol@example.com"},
			)

			requireEntraIDAccessListMembers(t, ctx, editorClient,
				gotGroups["group3"],
				[]string{"alice@example.com", "bob@example.com", "carol@example.com"},
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
