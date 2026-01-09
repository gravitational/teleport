package entraid

import (
	"context"
	"slices"

	"github.com/gravitational/trace"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	usersv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

// connectorName is the name of the saml connector to be used
// in the Entra ID plugin.
const connectorName = "entra-id"

func createEntraIDPlugin(ctx context.Context, authClient authclient.ClientI) error {
	request := &pluginspb.CreatePluginRequest{
		Plugin: &types.PluginV1{
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
							DefaultOwners: []string{"admin"},
							// CredentialsSource is not validated during tests.
							CredentialsSource: types.EntraIDCredentialsSource_ENTRAID_CREDENTIALS_SOURCE_SYSTEM_CREDENTIALS,
							SsoConnectorId:    connectorName,
							TenantId:          "bar",
							EntraAppId:        "app1", // matches app name available in default [msgraphtest.PayloadGetApplication].
						},
					},
				},
			},
		},
	}

	_, err := authClient.PluginsClient().CreatePlugin(ctx, request)
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
