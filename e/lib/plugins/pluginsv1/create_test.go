package pluginsv1

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	jamffake "github.com/gravitational/teleport/e/lib/jamf/fake"
	"github.com/gravitational/teleport/e/lib/jamf/testenv"
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

// withLabel generates a label-matching predicate function for use when with
// things like `slices.IndexFunc`
func withLabel[T types.ResourceWithLabels](key, requiredValue string) func(T) bool {
	return func(r T) bool {
		if actualValue, ok := r.GetLabel(key); ok {
			return actualValue == requiredValue
		}
		return false
	}
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
	staticCredentialsForBadOkta := &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: "static-creds-for-bad-okta",
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
		nil)
	badOkta := &types.PluginV1{
		Metadata: types.Metadata{Name: "bad-okta"},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Okta{},
		},
	}

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

	t.Run("valid request with multiple static credentials", func(t *testing.T) {
		// Given a plugin creation request with multiple static credentials
		// attached...
		credIdentityLabels := map[string]string{
			"id_label_1": "value1",
			"id_label_2": "value2",
		}
		req := &pluginspb.CreatePluginRequest{
			Plugin: oktaPlugin,
			StaticCredentialsList: []*types.PluginStaticCredentialsV1{
				staticCredentials,
				{
					ResourceHeader: types.ResourceHeader{
						Metadata: types.Metadata{
							Name: "scim-bearer-token",
							Labels: map[string]string{
								"okta/purpose": "scim-auth",
							},
						},
					},
					Spec: &types.PluginStaticCredentialsSpecV1{
						Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
							APIToken: "some-other-api-token",
						},
					},
				},
			},
			CredentialLabels: credIdentityLabels,
		}

		// When I attempt to create the plugin resource, expect that the
		// operation succeeds
		_, err := suite.svc.CreatePlugin(ctx, req)
		require.NoError(t, err)

		// Expect that the back-end plugin resource was created
		stored, err := suite.pluginService.GetPlugin(ctx, oktaPlugin.GetName(), true)
		require.NoError(t, err)

		// Expect that the back-end plugin resource's credential ref was
		// populated with the labels specified in the request
		credRefLabels := stored.GetCredentials().GetStaticCredentialsRef().Labels
		pluginLabel := credRefLabels[teleport.PluginLabel]
		require.NotEmpty(t, pluginLabel)

		expectedLabels := utils.CopyStringsMap(credIdentityLabels)
		expectedLabels[teleport.PluginLabel] = pluginLabel
		require.Equal(t, expectedLabels, credRefLabels)

		// When I query the plugin credentials by label...
		allCreds, err := suite.pluginStaticCredentialsService.GetPluginStaticCredentialsByLabels(ctx, credRefLabels)

		// Expect the operation to succeed and return two credentials
		require.NoError(t, err)
		require.Len(t, allCreds, 2)

		// Expect that one credential is the secondary cred, as identified by
		// the extra label given to it
		scimCredIdx := slices.IndexFunc(allCreds, withLabel[types.PluginStaticCredentials]("okta/purpose", "scim-auth"))
		require.NotEqual(t, -1, scimCredIdx)

		// When I delete the created plugin...
		_, err = suite.svc.DeletePlugin(ctx, &pluginspb.DeletePluginRequest{
			Name: oktaPlugin.GetName(),
		})
		require.NoError(t, err)

		// Expect the plugin record to no longer exist
		_, err = suite.pluginService.GetPlugin(ctx, oktaPlugin.GetName(), true)
		require.True(t, trace.IsNotFound(err))

		// Expect that the associated credentials, when requested by name, no
		// longer exist...
		for _, pluginCred := range allCreds {
			_, err = suite.pluginStaticCredentialsService.GetPluginStaticCredentials(ctx, pluginCred.GetName())
			require.True(t, trace.IsNotFound(err))
		}

		// Expect that the associated credentials, when queried by label, still
		// do not exist
		allCreds, err = suite.pluginStaticCredentialsService.GetPluginStaticCredentialsByLabels(ctx, credRefLabels)
		require.NoError(t, err)
		require.Empty(t, allCreds)
	})

	t.Run("bad request with static credentials", func(t *testing.T) {
		_, err := suite.svc.CreatePlugin(ctx, &pluginspb.CreatePluginRequest{
			Plugin:            badOkta,
			StaticCredentials: staticCredentialsForBadOkta,
		})
		require.Error(t, err)

		_, err = suite.pluginService.GetPlugin(ctx, badOkta.GetName(), true)
		require.True(t, trace.IsNotFound(err))

		_, err = suite.pluginStaticCredentialsService.GetPluginStaticCredentials(ctx, staticCredentialsForBadOkta.GetName())
		require.True(t, trace.IsNotFound(err))
	})
}

func TestService_CreatePlugin_jamf(t *testing.T) {
	t.Parallel()

	jamfEnv := testenv.NewUsingT(t, nil /* opts */)

	const username = "llama"
	const password = "secret"
	jamfEnv.API.SetUsers([]*jamffake.User{
		{
			Username: username,
			Password: password,
		},
	})

	suite := createSuite(t)
	suite.setRules([]types.Rule{
		{Resources: []string{types.KindPlugin}, Verbs: services.RW()},
	})
	service := suite.svc
	service.httpClient = jamfEnv.HTTPClient

	okPlugin := &types.PluginV1{
		SubKind: types.PluginSubkindMDM,
		Metadata: types.Metadata{
			Name: types.PluginTypeJamf,
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Jamf{
				Jamf: &types.PluginJamfSettings{
					JamfSpec: &types.JamfSpecV1{
						ApiEndpoint: jamfEnv.APIEndpoint,
					},
				},
			},
		},
	}
	okStaticCreds := &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: "jamf-static-credentials",
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_BasicAuth{
				BasicAuth: &types.PluginStaticCredentialsBasicAuth{
					Username: username, Password: password,
				},
			},
		},
	}

	ctx := context.Background()

	tests := []struct {
		name        string
		plugin      *types.PluginV1
		staticCreds *types.PluginStaticCredentialsV1
		wantErr     string
	}{
		{
			name:        "ok",
			plugin:      okPlugin,
			staticCreds: okStaticCreds,
		},
		{
			name: "bad plugin URL",
			plugin: func() *types.PluginV1 {
				cp := proto.Clone(okPlugin).(*types.PluginV1)
				cp.Spec.GetJamf().JamfSpec.ApiEndpoint = jamfEnv.APIEndpoint + "badllama"
				return cp
			}(),
			staticCreds: okStaticCreds,
			wantErr:     "verifying Jamf",
		},
		{
			name:   "bad plugin credentials",
			plugin: okPlugin,
			staticCreds: func() *types.PluginStaticCredentialsV1 {
				cp := proto.Clone(okStaticCreds).(*types.PluginStaticCredentialsV1)
				cp.Spec.GetBasicAuth().Username = "badllama"
				return cp
			}(),
			wantErr: "verifying Jamf",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.CreatePlugin(ctx, &pluginspb.CreatePluginRequest{
				Plugin:            test.plugin,
				StaticCredentials: test.staticCreds,
			})
			if test.wantErr == "" {
				assert.NoError(t, err, "CreatePlugin")
			} else {
				assert.ErrorContains(t, err, test.wantErr, "CreatePlugin error mismatch")
			}
			if err != nil {
				return
			}

			// Delete plugin after tests. Makes consecutive test cases simpler, but
			// otherwise this isn't part of the test scenario.
			_, err = service.DeletePlugin(ctx, &pluginspb.DeletePluginRequest{
				Name: test.plugin.GetName(),
			})
			assert.NoError(t, err, "DeletePlugin")
		})
	}
}
