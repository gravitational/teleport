package entraid

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/gravitational/trace"

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

func newTestEnv(t *testing.T, storage *msgraphtest.Storage) testEnv {
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

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.EntraIDSAMLConnector(connectorName)),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "user-editor", "editor"),
		common.WithHTTPClient(httpClient.Transport),
	)

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
						EntraAppId:        "app1", // matches app name available in default [msgraphtest.PayloadGetApplication].
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
