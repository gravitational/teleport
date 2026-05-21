package entraid

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"

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

// connectorName is the name of the saml connector to be used
// in the Entra ID plugin.
const connectorName = "entra-id"

type testEnv struct {
	fakeServer *msgraphtest.Server
	authClient authclient.ClientI
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
	}
}

var defaultOwner = accesslist.Owner{
	Name:             "admin",
	MembershipKind:   accesslistv1.MembershipKind_MEMBERSHIP_KIND_USER.String(),
	IneligibleStatus: accesslistv1.IneligibleStatus_INELIGIBLE_STATUS_ELIGIBLE.String(),
}

func newDefaultPluginSpec(t *testing.T) *types.PluginV1 {
	t.Helper()
	return &types.PluginV1{
		Metadata: types.Metadata{
			Name: types.PluginTypeEntraID,
			Labels: map[string]string{
				types.HostedPluginLabel: "true",
			},
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_EntraId{
				EntraId: &types.PluginEntraIDSettings{
					SyncSettings: &types.PluginEntraIDSyncSettings{
						DefaultOwners: []string{defaultOwner.Name},
						// CredentialsSource is not validated during tests.
						CredentialsSource: types.EntraIDCredentialsSource_ENTRAID_CREDENTIALS_SOURCE_SYSTEM_CREDENTIALS,
						SsoConnectorId:    connectorName,
						TenantId:          "bar",
						EntraAppId:        app1ID, // matches app name available in newDefaultStorage()
					},
				},
			},
		},
	}
}

func createEntraIDPlugin(ctx context.Context, authClient authclient.ClientI, plugin *types.PluginV1) error {
	_, err := authClient.PluginsClient().CreatePlugin(ctx, &pluginspb.CreatePluginRequest{Plugin: plugin})
	return trace.Wrap(err)
}

func listEntraIDUsers(ctx context.Context, authClient authclient.ClientI) ([]string, error) {
	resp, err := authClient.ListUsers(ctx, &usersv1.ListUsersRequest{
		WithSecrets: false,
		Filter: &types.UserFilter{
			SearchKeywords:  []string{types.OriginEntraID}, // searches entra id origin label.
			SkipSystemUsers: true,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := make([]string, 0, len(resp.Users))
	for _, u := range resp.Users {
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

func listEntraIDMembers(ctx context.Context, accessListName string, aclClient services.AccessLists) ([]string, error) {
	var out []string

	fn := func(ctx context.Context, pageSize int, nextToken string) ([]*accesslist.AccessListMember, string, error) {
		return aclClient.ListAccessListMembers(ctx, accessListName, pageSize, nextToken)
	}
	members, err := stream.Collect(clientutils.Resources(ctx, fn))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, m := range members {
		// members are expected to have entra id origin label
		if m.Origin() != types.OriginEntraID {
			continue
		}
		out = append(out, m.GetName())
	}

	slices.Sort(out)
	return out, nil
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
