package pluginsv1

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport-plugins/access/common/auth/storage"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type mockExchanger struct {
	exchange func(authorizationCode, redirectURI string) (*storage.Credentials, error)
}

func (m *mockExchanger) Exchange(ctx context.Context, authorizationCode string, redirectURI string) (*storage.Credentials, error) {
	return m.exchange(authorizationCode, redirectURI)
}

func TestPluginCreate(t *testing.T) {
	t.Parallel()

	const pluginName = "slack-default"
	const validAuthCode = "123456"
	const invalidAuthCode = "654321"
	const validRedirectURI = "https://foo.localhost/callback"
	const invalidRedirectURI = "https://mallory.com/callback"

	suite := createSuite(t)
	suite.setRules([]types.Rule{
		{Resources: []string{types.KindPlugin}, Verbs: services.RW()},
	})

	validBootstrapCredentials := &types.PluginBootstrapCredentialsV1{
		Credentials: &types.PluginBootstrapCredentialsV1_Oauth2AuthorizationCode{
			Oauth2AuthorizationCode: &types.PluginOAuth2AuthorizationCodeCredentials{
				AuthorizationCode: validAuthCode,
				RedirectUri:       validRedirectURI,
			},
		},
	}
	invalidBootstrapCredentials := &types.PluginBootstrapCredentialsV1{
		Credentials: &types.PluginBootstrapCredentialsV1_Oauth2AuthorizationCode{
			Oauth2AuthorizationCode: &types.PluginOAuth2AuthorizationCodeCredentials{
				AuthorizationCode: invalidAuthCode,
				RedirectUri:       invalidRedirectURI,
			},
		},
	}

	plugin := types.NewPluginV1(
		types.Metadata{Name: "slack-default"},
		types.PluginSpecV1{
			Settings: &types.PluginSpecV1_SlackAccessPlugin{
				SlackAccessPlugin: &types.PluginSlackAccessSettings{
					FallbackChannel: "#general",
				},
			},
		},
		nil)

	exchangedCreds := &storage.Credentials{
		AccessToken:  "my-access-token",
		RefreshToken: "my-refresh-token",
		ExpiresAt:    time.Now().UTC().Add(6 * time.Hour),
	}

	suite.exchangers.Slack = &mockExchanger{
		exchange: func(authCode string, redirectURI string) (*storage.Credentials, error) {
			if authCode == validAuthCode && redirectURI == validRedirectURI {
				return exchangedCreds, nil
			}
			return nil, trace.AccessDenied("invalid parameters")
		},
	}

	ctx := context.Background()

	t.Run("empty plugin in request", func(t *testing.T) {
		_, err := suite.svc.CreatePlugin(ctx, &pluginspb.CreatePluginRequest{
			BootstrapCredentials: validBootstrapCredentials,
		})
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err))
	})

	t.Run("empty bootstrap credentials in request", func(t *testing.T) {
		_, err := suite.svc.CreatePlugin(ctx, &pluginspb.CreatePluginRequest{
			Plugin: plugin,
		})
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err))
	})

	t.Run("unsupported plugin type", func(t *testing.T) {
		_, err := suite.svc.CreatePlugin(ctx, &pluginspb.CreatePluginRequest{
			BootstrapCredentials: validBootstrapCredentials,
			Plugin:               &types.PluginV1{},
		})
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err))
	})

	t.Run("invalid bootstrap credentials", func(t *testing.T) {
		_, err := suite.svc.CreatePlugin(ctx, &pluginspb.CreatePluginRequest{
			Plugin:               plugin,
			BootstrapCredentials: invalidBootstrapCredentials,
		})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("valid request", func(t *testing.T) {
		_, err := suite.svc.CreatePlugin(ctx, &pluginspb.CreatePluginRequest{
			Plugin:               plugin,
			BootstrapCredentials: validBootstrapCredentials,
		})
		require.NoError(t, err)

		stored, err := suite.backendService.GetPlugin(ctx, pluginName, true)
		require.NoError(t, err)
		creds := stored.GetCredentials().GetOauth2AccessToken()
		require.Equal(t, exchangedCreds.AccessToken, creds.AccessToken)
		require.Equal(t, exchangedCreds.RefreshToken, creds.RefreshToken)
		require.Equal(t, exchangedCreds.ExpiresAt, creds.Expires)
	})
}
