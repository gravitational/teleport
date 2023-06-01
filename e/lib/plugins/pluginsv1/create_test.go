package pluginsv1

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/plugins"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/integrations/access/common/auth/storage"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

type mockAuthorizer struct {
	exchange func(authorizationCode, redirectURI string) (*storage.Credentials, error)
}

func (m *mockAuthorizer) Exchange(ctx context.Context, authorizationCode string, redirectURI string) (*storage.Credentials, error) {
	return m.exchange(authorizationCode, redirectURI)
}

func (m *mockAuthorizer) Refresh(ctx context.Context, refreshToken string) (*storage.Credentials, error) {
	return nil, trace.NotImplemented("Refresh() not used by the test")
}

func TestPluginCreateDelete(t *testing.T) {
	t.Parallel()

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
	staticCredentials := &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: "static-creds",
				Labels: map[string]string{
					"label1":                          "value1",
					"label2":                          "value2",
					types.TeleportInternalLabelPrefix: "filtered",
				},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
				APIToken: "some-token",
			},
		},
	}

	slackPlugin := types.NewPluginV1(
		types.Metadata{Name: "slack-default"},
		types.PluginSpecV1{
			Settings: &types.PluginSpecV1_SlackAccessPlugin{
				SlackAccessPlugin: &types.PluginSlackAccessSettings{
					FallbackChannel: "#general",
				},
			},
		},
		nil)
	oktaPlugin := types.NewPluginV1(
		types.Metadata{Name: "okta-default"},
		types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Okta{
				Okta: &types.PluginOktaSettings{
					OrgUrl: "https://www.okta.com",
				},
			},
		},
		// TODO(mdwn): Remove this once the bearer token is no longer needed for the Okta plugin.
		&types.PluginCredentialsV1{
			Credentials: &types.PluginCredentialsV1_BearerToken{
				BearerToken: &types.PluginBearerTokenCredentials{
					Token: "bearer-token",
				},
			},
		})

	exchangedCreds := &storage.Credentials{
		AccessToken:  "my-access-token",
		RefreshToken: "my-refresh-token",
		ExpiresAt:    time.Now().UTC().Add(6 * time.Hour),
	}

	slackAuthorizer := &mockAuthorizer{
		exchange: func(authCode string, redirectURI string) (*storage.Credentials, error) {
			if authCode == validAuthCode && redirectURI == validRedirectURI {
				return exchangedCreds, nil
			}
			return nil, trace.AccessDenied("invalid parameters")
		},
	}

	suite.pluginAuthorizers.Add(types.PluginTypeSlack, &plugins.Authorizer{Authorizer: slackAuthorizer, ClientID: "123456"})

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
			Plugin: slackPlugin,
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
			Plugin:               slackPlugin,
			BootstrapCredentials: invalidBootstrapCredentials,
		})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("valid request", func(t *testing.T) {
		_, err := suite.svc.CreatePlugin(ctx, &pluginspb.CreatePluginRequest{
			Plugin:               slackPlugin,
			BootstrapCredentials: validBootstrapCredentials,
		})
		require.NoError(t, err)

		stored, err := suite.pluginService.GetPlugin(ctx, slackPlugin.GetName(), true)
		require.NoError(t, err)
		creds := stored.GetCredentials().GetOauth2AccessToken()
		require.Equal(t, exchangedCreds.AccessToken, creds.AccessToken)
		require.Equal(t, exchangedCreds.RefreshToken, creds.RefreshToken)
		require.Equal(t, exchangedCreds.ExpiresAt, creds.Expires)
	})

	t.Run("valid request with static credentials", func(t *testing.T) {
		_, err := suite.svc.CreatePlugin(ctx, &pluginspb.CreatePluginRequest{
			Plugin:            oktaPlugin,
			StaticCredentials: staticCredentials,
		})
		require.NoError(t, err)

		stored, err := suite.pluginService.GetPlugin(ctx, oktaPlugin.GetName(), true)
		require.NoError(t, err)

		credRefLabels := stored.GetCredentials().GetStaticCredentialsRef().Labels
		pluginLabel := credRefLabels[teleport.PluginLabel]
		require.NotEmpty(t, pluginLabel)

		expectedLabels := utils.CopyStringsMap(staticCredentials.GetStaticLabels())
		for k := range expectedLabels {
			if strings.HasPrefix(k, types.TeleportInternalLabelPrefix) {
				delete(expectedLabels, k)
			}
		}
		expectedLabels[teleport.PluginLabel] = pluginLabel
		require.Equal(t, expectedLabels, credRefLabels)

		allCreds, err := suite.pluginStaticCredentialsService.GetPluginStaticCredentialsByLabels(ctx, credRefLabels)
		require.NoError(t, err)
		require.Len(t, allCreds, 1)

		pluginCredentialsName := allCreds[0].GetName()

		_, err = suite.svc.DeletePlugin(ctx, &pluginspb.DeletePluginRequest{
			Name: oktaPlugin.GetName(),
		})
		require.NoError(t, err)

		_, err = suite.pluginService.GetPlugin(ctx, oktaPlugin.GetName(), true)
		require.True(t, trace.IsNotFound(err))

		_, err = suite.pluginStaticCredentialsService.GetPluginStaticCredentials(ctx, pluginCredentialsName)
		require.True(t, trace.IsNotFound(err))

		allCreds, err = suite.pluginStaticCredentialsService.GetPluginStaticCredentialsByLabels(ctx, credRefLabels)
		require.NoError(t, err)
		require.Empty(t, allCreds)
	})
}
