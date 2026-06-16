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
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	usersv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/msgraph/models"
	"github.com/gravitational/teleport/lib/msgraph/msgraphtest"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// connectorName is the name of the saml connector to be used
	// in the Entra ID plugin.
	connectorName = "entra-id"

	pluginName = types.PluginTypeEntraID

	deltaSyncInterval = 15 * time.Second
)

type testEnv struct {
	fakeServer *msgraphtest.Server
	authClient authclient.ClientI
	sut        *common.SUT
}

func newTestEnv(t *testing.T, storage *msgraphtest.Storage, opts ...common.Option) testEnv {
	fakeServer := msgraphtest.NewServer(
		msgraphtest.WithStorage(storage),
	)
	t.Cleanup(fakeServer.TLSServer.Close)

	httpClient := &http.Client{
		Transport: &msgraphtest.RewriteTransport{
			Base: fakeServer.TLSServer.Client().Transport,
			URL:  mustParseURL(t, fakeServer.TLSServer.URL),
		},
	}

	opts = append(opts,
		common.WithSAMLConnector(idp.EntraIDSAMLConnector(connectorName, group3ID)),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "user-editor", "editor"),
		common.WithHTTPClient(httpClient.Transport))

	sut := common.InitSUT(t, opts...)

	return testEnv{
		fakeServer: fakeServer,
		authClient: sut.GetClusterClientForUser(t, "user-editor").AuthClient,
		sut:        sut,
	}
}

var defaultOwner = accesslist.Owner{
	Name:             "user-editor",
	MembershipKind:   accesslistv1.MembershipKind_MEMBERSHIP_KIND_USER.String(),
	IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String(),
}

type pluginSyncSettingsOpt func(*types.PluginEntraIDSyncSettings)

func withSyncIntervals(intervals *types.PluginEntraIDSyncIntervals) pluginSyncSettingsOpt {
	return func(settings *types.PluginEntraIDSyncSettings) {
		settings.SyncIntervals = intervals
	}
}

func withAclOwnersConfig(source types.EntraIDAccessListOwnersSource) pluginSyncSettingsOpt {
	return func(settings *types.PluginEntraIDSyncSettings) {
		settings.AccessListOwnersSource = source
	}
}

func withGroupFilters(filters []*types.PluginSyncFilter) pluginSyncSettingsOpt {
	return func(settings *types.PluginEntraIDSyncSettings) {
		settings.GroupFilters = filters
	}
}

func newDefaultPluginSpec(t *testing.T, opts ...pluginSyncSettingsOpt) *types.PluginV1 {
	t.Helper()

	syncSettings := &types.PluginEntraIDSyncSettings{
		DefaultOwners: []string{defaultOwner.Name},
		// CredentialsSource is not validated during tests.
		CredentialsSource: types.EntraIDCredentialsSource_ENTRAID_CREDENTIALS_SOURCE_SYSTEM_CREDENTIALS,
		SsoConnectorId:    connectorName,
		TenantId:          "bar",
		EntraAppId:        app1ID, // matches app name available in newDefaultStorage()
		SyncIntervals: &types.PluginEntraIDSyncIntervals{
			Full: "15s",
		},
	}

	for _, opt := range opts {
		opt(syncSettings)
	}

	return &types.PluginV1{
		Metadata: types.Metadata{
			Name: pluginName,
			Labels: map[string]string{
				types.HostedPluginLabel: "true",
			},
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_EntraId{
				EntraId: &types.PluginEntraIDSettings{
					SyncSettings: syncSettings,
				},
			},
		},
	}
}

func createEntraIDPlugin(ctx context.Context, authClient authclient.ClientI, plugin *types.PluginV1) error {
	_, err := authClient.PluginsClient().CreatePlugin(ctx, pluginspb.CreatePluginRequest_builder{Plugin: plugin}.Build())
	return trace.Wrap(err)
}

func mustCreatePlugin(t *testing.T, ctx context.Context, authClient authclient.ClientI, opts ...pluginSyncSettingsOpt) {
	t.Helper()

	plugin := newDefaultPluginSpec(t, opts...)
	err := createEntraIDPlugin(ctx, authClient, plugin)
	require.NoError(t, err, "expected Entra ID plugin to be created")
}

func updateEntraIDPlugin(ctx context.Context, authClient authclient.ClientI, plugin *types.PluginV1) error {
	_, err := authClient.PluginsClient().UpdatePlugin(ctx, pluginspb.UpdatePluginRequest_builder{Plugin: plugin}.Build())
	return trace.Wrap(err)
}

func listEntraIDUsers(ctx context.Context, authClient authclient.ClientI) ([]string, error) {
	resp, err := authClient.ListUsers(ctx, usersv1.ListUsersRequest_builder{
		WithSecrets: false,
		Filter: &types.UserFilter{
			SearchKeywords:  []string{types.OriginEntraID}, // searches entra id origin label.
			SkipSystemUsers: true,
		},
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := make([]string, 0, len(resp.GetUsers()))
	for _, u := range resp.GetUsers() {
		out = append(out, u.GetName())
	}

	return out, nil
}

func listEntraIDAccessLists(ctx context.Context, aclClient services.AccessLists) (map[string]*accesslist.AccessList, error) {
	out := make(map[string]*accesslist.AccessList)
	fn := func(ctx context.Context, pageSize int, nextToken string) ([]*accesslist.AccessList, string, error) {
		return aclClient.ListAccessLists(ctx, pageSize, nextToken)
	}
	acls, err := stream.Collect(clientutils.Resources(ctx, fn))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, a := range acls {
		if a.Origin() != types.OriginEntraID {
			continue
		}
		out[a.Spec.Title] = a
	}

	return out, nil
}

func listEntraIDMembers(ctx context.Context, accessListName string, aclClient services.AccessLists) ([]*accesslist.AccessListMember, error) {
	fn := func(ctx context.Context, pageSize int, nextToken string) ([]*accesslist.AccessListMember, string, error) {
		return aclClient.ListAccessListMembers(ctx, accessListName, pageSize, nextToken)
	}
	members, err := stream.Collect(clientutils.Resources(ctx, fn))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return members, nil
}

func aclOwners(t *assert.CollectT, users []*models.User) []accesslist.Owner {
	t.Helper()
	out := []accesslist.Owner{}
	for _, u := range users {
		out = append(out, accesslist.Owner{
			Name:             *u.UserPrincipalName,
			MembershipKind:   accesslistv1.MembershipKind_MEMBERSHIP_KIND_USER.String(),
			IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String(),
		})
	}

	return out
}

// newEntraGroup returns a new Entra ID group.
func newEntraGroup(id, name string) *models.Group {
	return &models.Group{
		DirectoryObject: models.DirectoryObject{
			ID:          to.Ptr(id),
			DisplayName: to.Ptr(name),
		},
		GroupTypes: []string{types.EntraIDSecurityGroups},
	}
}

// newEntraUser returns a new Entra ID user.
func newEntraUser(id, mail, display string) *models.User {
	return &models.User{
		DirectoryObject: models.DirectoryObject{
			ID:          to.Ptr(id),
			DisplayName: to.Ptr(display),
		},
		UserPrincipalName: to.Ptr(mail),
		Mail:              to.Ptr(mail),
	}
}

const (
	aliceID = "2765d9b2-a70c-4d30-a1ec-f02c40fcf4ad"
	bobID   = "aace3f26-9f57-4519-b5fb-0d38fe93d3c2"
	carolID = "1c5f5517-27dc-415f-9793-c9531cd17d48"

	group1ID = "fdfc6317-cc24-4c9c-b32a-143b0fbf3cd0"
	group2ID = "7b1e66cc-3768-4281-bc4d-b84720654842"
	group3ID = "4698ee2a-bf74-467e-8bde-63db8f323a44"

	app1ID = "0e0038e9-6653-4701-8c44-826afbbc39f6"
)

var (
	defaultUsernames         = []string{"alice@example.com", "bob@example.com", "carol@example.com"}
	defaultGroupDisplayNames = []string{"group1", "group2", "group3"}
)

// newDefaultStorage creates a new msgraphtest.Storage with hardcoded test data.
func newDefaultStorage() *msgraphtest.Storage {
	storage := msgraphtest.NewStorage()

	alice := newEntraUser(aliceID, "alice@example.com", "Alice A")
	storage.Users[aliceID] = alice
	bob := newEntraUser(bobID, "bob@example.com", "Bob B")
	storage.Users[bobID] = bob
	carol := newEntraUser(carolID, "carol@example.com", "Carol C")
	storage.Users[carolID] = carol

	group1 := newEntraGroup(group1ID, "group1")
	storage.Groups[group1ID] = group1
	group2 := newEntraGroup(group2ID, "group2")
	storage.Groups[group2ID] = group2
	group3 := newEntraGroup(group3ID, "group3")
	storage.Groups[group3ID] = group3

	storage.GroupMembers[group1ID] = []models.GroupMember{alice, group2}
	storage.GroupMembers[group2ID] = []models.GroupMember{alice, bob, carol}
	storage.GroupMembers[group3ID] = []models.GroupMember{alice, bob, carol}

	storage.GroupOwners[group1ID] = []*models.User{alice, bob}
	storage.GroupOwners[group3ID] = []*models.User{bob, carol}

	app1 := &models.Application{
		DirectoryObject: models.DirectoryObject{
			ID:          to.Ptr("ddca8610-0fa7-4acf-a80a-4d3b9c8346b9"),
			DisplayName: to.Ptr("test SAML App"),
		},
		AppID: to.Ptr(app1ID),
	}
	storage.Applications[app1ID] = app1

	return storage
}

func expectDefaultResources(t *testing.T, authClient authclient.ClientI) {
	t.Helper()

	expectDefaultPluginStatus(t, authClient, pluginName)
	expectDefaultUserSync(t, authClient)
	expectDefaultGroupSync(t, authClient)
	expectDefaultGroupOwners(t, authClient)
}

func expectDefaultResourcesWithEntraOwners(t *testing.T, ctx context.Context, authClient authclient.ClientI) {
	t.Helper()

	expectDefaultPluginStatus(t, authClient, pluginName)
	expectDefaultUserSync(t, authClient)
	expectDefaultGroupSync(t, authClient)
	expectDefaultEntraOwners(t, ctx, authClient)
}

func expectDefaultResourcesWithPluginAndEntraOwners(t *testing.T, ctx context.Context, authClient authclient.ClientI) {
	t.Helper()

	expectDefaultPluginStatus(t, authClient, pluginName)
	expectDefaultUserSync(t, authClient)
	expectDefaultGroupSync(t, authClient)
	expectDefaultEntraAndPluginOwner(t, ctx, authClient.AccessListClient())
}

// expectPluginStatusUpdated checks for the plugin status to be updated.
func expectPluginStatusUpdated(t *testing.T, ctx context.Context, authClt authclient.ClientI, name string, clock *clockwork.FakeClock) {
	t.Helper()

	plugin, err := authClt.PluginsClient().GetPlugin(ctx, pluginspb.GetPluginRequest_builder{
		Name: name,
	}.Build())
	if err != nil {
		t.Fatalf("failed to get entra id plugin %q", name)
	}

	// Record current plugin before advancing the clock.
	before := plugin.GetStatus().GetLastSyncTime()
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		waitCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()
		if err := clock.BlockUntilContext(waitCtx, 1); err == nil {
			clock.Advance(time.Second)
		}
		plugin, err := authClt.PluginsClient().GetPlugin(ctx, pluginspb.GetPluginRequest_builder{
			Name: name,
		}.Build())
		require.NoError(t, err)
		after := plugin.GetStatus().GetLastSyncTime()
		require.False(t, after.IsZero(), "plugin status LastSyncTime is not set")
		require.True(t, after.After(before), "expected a new Entra ID sync to complete with new last sync time")
	}, 15*time.Second, 30*time.Millisecond)
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
			updatedPlugin, err := authClt.PluginsClient().GetPlugin(ctx, pluginspb.GetPluginRequest_builder{
				Name: name,
			}.Build())
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

func mustDeleteUsersFromTeleport(t *testing.T, authClient authclient.ClientI, users []string) {
	t.Helper()

	for _, name := range users {
		err := authClient.DeleteUser(t.Context(), name)
		require.NoError(t, err, "user not deleted")
	}
}

func mustDeleteAccessLists(t *testing.T, authClient authclient.ClientI, acls []string) {
	t.Helper()

	for _, name := range acls {
		err := authClient.AccessListClient().DeleteAccessList(t.Context(), name)
		require.NoError(t, err, "Access List not deleted")
	}
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

	gotMembers := mustListMembers(t, ctx, acl.GetName(), authClient.AccessListClient())
	require.NotNil(t, gotMembers, "expected membership to not to be nil")

	require.ElementsMatch(t, expectedMembers, gotMembers, "expected Entra ID group members to match. acl=%s", acl.Spec.Title)
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

func mustListMembers(t *assert.CollectT, ctx context.Context, accessListName string, aclClient services.AccessLists) []string {
	t.Helper()

	members, err := listEntraIDMembers(ctx, accessListName, aclClient)
	require.NoError(t, err)

	out := make([]string, 0, len(members))
	for _, m := range members {
		out = append(out, m.GetName())
	}
	slices.Sort(out)
	return out
}

// expectDefaultEntraOwners checks for Entra ID group owners as configured in the newDefaultStorage func.
func expectDefaultEntraOwners(t *testing.T, ctx context.Context, authClient authclient.ClientI) {
	t.Helper()

	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			entraOwners := newDefaultStorage().GroupOwners
			gotAccesslists, err := listEntraIDAccessLists(ctx, authClient.AccessListClient())
			require.NoError(t, err, "listing entra id access lists")

			expectedGroup1Owners := aclOwners(t, entraOwners[group1ID])
			compareOwners(t, "group1", expectedGroup1Owners, gotAccesslists["group1"].Spec.Owners)

			// group2 has zero owners, should fallback to default owners.
			compareOwners(t, "group2", []accesslist.Owner{defaultOwner}, gotAccesslists["group2"].Spec.Owners)

			expectedGroup3Owners := aclOwners(t, entraOwners[group3ID])
			compareOwners(t, "group3", expectedGroup3Owners, gotAccesslists["group3"].Spec.Owners)

		},
		time.Second*15, time.Millisecond*30)
}

func mustGetUserRevisions(t *testing.T, ctx context.Context, authClient authclient.ClientI, names []string) []string {
	t.Helper()

	out := make([]string, 0, len(names))
	for _, name := range names {
		user, err := authClient.GetUser(ctx, name, false)
		require.NoError(t, err)

		if user.Origin() == types.OriginEntraID {
			out = append(out, user.GetRevision())
		}
	}
	return out
}

type accessListWithMembersRevision struct {
	AccessListRevision string
	MembersRevision    []string
}

func mustGetAclWithMembersRevision(t *testing.T, ctx context.Context, authClient authclient.ClientI, names []string) []accessListWithMembersRevision {
	t.Helper()

	out := make([]accessListWithMembersRevision, 0, len(names))
	for _, name := range names {
		acl, err := authClient.AccessListClient().GetAccessList(ctx, name)
		require.NoError(t, err)
		if acl.Origin() != types.OriginEntraID {
			continue
		}

		members, err := listEntraIDMembers(ctx, name, authClient.AccessListClient())
		require.NoError(t, err)

		membersRevision := []string{}
		for _, m := range members {
			if m.Origin() != types.OriginEntraID {
				continue
			}
			membersRevision = append(membersRevision, m.GetRevision())
		}
		slices.Sort(membersRevision)

		out = append(out, accessListWithMembersRevision{
			AccessListRevision: acl.GetRevision(),
			MembersRevision:    membersRevision,
		})
	}
	return out
}

func expectDefaultEntraAndPluginOwner(t *testing.T, ctx context.Context, clt services.AccessLists) {
	t.Helper()

	require.EventuallyWithT(t,
		func(t *assert.CollectT) {
			gotAccesslists, err := listEntraIDAccessLists(ctx, clt)
			require.NoError(t, err, "listing entra id access lists")

			defaultStorageOwners := newDefaultStorage().GroupOwners
			defaultOwners := []accesslist.Owner{defaultOwner}
			// Merge Entra ID group owners and plugin default owners.
			expectedGroup1Owners := slices.Concat(aclOwners(t, defaultStorageOwners[group1ID]), defaultOwners)
			compareOwners(t, "group1", expectedGroup1Owners, gotAccesslists["group1"].Spec.Owners)

			// group2 has zero owners, should fallback to default owners.
			compareOwners(t, "group2", defaultOwners, gotAccesslists["group2"].Spec.Owners)

			// Merge Entra ID group owners and plugin default owners.
			expectedGroup3Owners := slices.Concat(aclOwners(t, defaultStorageOwners[group3ID]), defaultOwners)
			compareOwners(t, "group3", expectedGroup3Owners, gotAccesslists["group3"].Spec.Owners)
		},
		time.Second*10, time.Millisecond*30)
}

func mustGetAclNames(t *testing.T, ctx context.Context, aclClient services.AccessLists, titles []string) []string {
	t.Helper()

	out := []string{}
	gotAccesslists, err := listEntraIDAccessLists(ctx, aclClient)
	require.NoError(t, err, "list entra access lists")
	for _, v := range gotAccesslists {
		if slices.Contains(titles, v.Spec.Title) {
			out = append(out, v.GetName())
		}
	}
	require.Len(t, out, len(titles), "expected all requested Entra ID Access lists to exists")
	return out
}

func waitForPluginStatusUpdate(t *testing.T, w types.Watcher, before time.Time) time.Time {
	t.Helper()

	var newLastSyncTime time.Time
	common.WaitForPutEvent(t, w, func(r types.Plugin) bool {
		if r.GetName() != pluginName {
			return false
		}
		newLastSyncTime = r.GetStatus().GetLastSyncTime()
		return !newLastSyncTime.IsZero() && newLastSyncTime.After(before)
	})
	return newLastSyncTime
}
