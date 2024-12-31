package pluginsv1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func TestPluginUpdate(t *testing.T) {
	suite := createSuite(t)
	ctx := context.Background()
	plugin := &types.PluginV1{
		Metadata: types.Metadata{Name: "okta"},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Okta{
				Okta: &types.PluginOktaSettings{
					OrgUrl: "https://okta.com",
					SyncSettings: &types.PluginOktaSyncSettings{
						SyncUsers:      true,
						SsoConnectorId: "conn_id",
					},
				},
			},
		},
	}

	suite.setRules([]types.Rule{{Resources: []string{types.KindPlugin}, Verbs: services.RO()}})
	_, err := suite.svc.UpdatePlugin(ctx, &pluginsv1.UpdatePluginRequest{
		Plugin: plugin,
	})
	require.True(t, trace.IsAccessDenied(err))

	suite.setRules([]types.Rule{{Resources: []string{types.KindPlugin}, Verbs: services.RW()}})

	_, err = suite.svc.UpdatePlugin(ctx, &pluginsv1.UpdatePluginRequest{
		Plugin: plugin,
	})
	require.True(t, trace.IsNotFound(err))

	_, err = suite.svc.CreatePlugin(ctx, &pluginsv1.CreatePluginRequest{
		Plugin: plugin,
		StaticCredentials: &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: "okta",
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: "api_token",
				},
			},
		},
	})
	require.NoError(t, err)

	p, err := suite.svc.GetPlugin(ctx, &pluginsv1.GetPluginRequest{
		Name:        "okta",
		WithSecrets: false,
	})
	require.NoError(t, err)

	p.Spec.GetOkta().SsoConnectorId = "new_sso_id"

	got, err := suite.svc.UpdatePlugin(ctx, &pluginsv1.UpdatePluginRequest{
		Plugin: p,
	})
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(p, got, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	// Test that disabled plugins can't be updated.
	suite.svc.disabledPlugins = []types.PluginType{plugin.GetType()}
	got, err = suite.svc.UpdatePlugin(ctx, &pluginsv1.UpdatePluginRequest{
		Plugin: got,
	})
	require.Error(t, err)
	require.Nil(t, got)
	require.True(t, trace.IsBadParameter(err))
}
