/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestPluginWithoutSecrets(t *testing.T) {
	spec := PluginSpecV1{
		Settings: &PluginSpecV1_SlackAccessPlugin{
			SlackAccessPlugin: &PluginSlackAccessSettings{
				FallbackChannel: "#access-requests",
			},
		},
	}

	creds := &PluginCredentialsV1{
		Credentials: &PluginCredentialsV1_Oauth2AccessToken{
			Oauth2AccessToken: &PluginOAuth2AccessTokenCredentials{
				AccessToken:  "access_token",
				RefreshToken: "refresh_token",
				Expires:      time.Now().UTC(),
			},
		},
	}

	plugin := NewPluginV1(Metadata{Name: "foobar"}, spec, creds)
	plugin = plugin.WithoutSecrets().(*PluginV1)
	require.Nil(t, plugin.Credentials)
}

func TestPluginOpenAIValidation(t *testing.T) {
	spec := PluginSpecV1{
		Settings: &PluginSpecV1_Openai{},
	}
	testCases := []struct {
		name      string
		creds     *PluginCredentialsV1
		assertErr require.ErrorAssertionFunc
	}{
		{
			name:  "no credentials",
			creds: nil,
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), "credentials must be set")
			},
		},
		{
			name:  "no credentials inner",
			creds: &PluginCredentialsV1{},
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), "must be used with the bearer token credential type")
			},
		},
		{
			name: "invalid credential type (oauth2)",
			creds: &PluginCredentialsV1{
				Credentials: &PluginCredentialsV1_Oauth2AccessToken{},
			},
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), "must be used with the bearer token credential type")
			},
		},
		{
			name: "valid credentials (token)",
			creds: &PluginCredentialsV1{
				Credentials: &PluginCredentialsV1_BearerToken{
					BearerToken: &PluginBearerTokenCredentials{
						Token: "xxx-abc",
					},
				},
			},
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			plugin := NewPluginV1(Metadata{Name: "foobar"}, spec, tc.creds)
			tc.assertErr(t, plugin.CheckAndSetDefaults())
		})
	}
}

func TestPluginOpsgenieValidation(t *testing.T) {
	testCases := []struct {
		name      string
		settings  *PluginSpecV1_Opsgenie
		creds     *PluginCredentialsV1
		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "no settings",
			settings: &PluginSpecV1_Opsgenie{
				Opsgenie: nil,
			},
			creds: nil,
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), "missing opsgenie settings")
			},
		},
		{
			name: "no api endpint",
			settings: &PluginSpecV1_Opsgenie{
				Opsgenie: &PluginOpsgenieAccessSettings{},
			},
			creds: nil,
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), "api endpoint url must be set")
			},
		},
		{
			name: "no static credentials",
			settings: &PluginSpecV1_Opsgenie{
				Opsgenie: &PluginOpsgenieAccessSettings{
					ApiEndpoint: "https://test.opsgenie.com",
				},
			},
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), "must be used with the static credentials ref type")
			},
		},
		{
			name: "static credentials labels not defined",
			settings: &PluginSpecV1_Opsgenie{
				Opsgenie: &PluginOpsgenieAccessSettings{
					ApiEndpoint: "https://test.opsgenie.com",
				},
			},
			creds: &PluginCredentialsV1{
				Credentials: &PluginCredentialsV1_StaticCredentialsRef{
					&PluginStaticCredentialsRef{
						Labels: map[string]string{},
					},
				},
			},
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), "labels must be specified")
			},
		},
		{
			name: "valid credentials (static credentials)",
			settings: &PluginSpecV1_Opsgenie{
				Opsgenie: &PluginOpsgenieAccessSettings{
					ApiEndpoint: "https://test.opsgenie.com",
				},
			},
			creds: &PluginCredentialsV1{
				Credentials: &PluginCredentialsV1_StaticCredentialsRef{
					&PluginStaticCredentialsRef{
						Labels: map[string]string{
							"label1": "value1",
						},
					},
				},
			},
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			plugin := NewPluginV1(Metadata{Name: "foobar"}, PluginSpecV1{
				Settings: tc.settings,
			}, tc.creds)
			tc.assertErr(t, plugin.CheckAndSetDefaults())
		})
	}
}

func requireBadParameterWith(msg string) require.ErrorAssertionFunc {
	return func(t require.TestingT, err error, args ...interface{}) {
		require.True(t, trace.IsBadParameter(err), "error: %v", err)
		require.Contains(t, err.Error(), msg)
	}
}

func TestPluginOktaValidation(t *testing.T) {
	validSettings := &PluginSpecV1_Okta{
		Okta: &PluginOktaSettings{
			OrgUrl:         "https://test.okta.com",
			EnableUserSync: true,
			SsoConnectorId: "some-sso-connector-id",
		},
	}

	validSettingsWithSyncSettings := &PluginSpecV1_Okta{
		Okta: &PluginOktaSettings{
			OrgUrl:         "https://test.okta.com",
			EnableUserSync: true,
			SsoConnectorId: "some-sso-connector-id",
			SyncSettings: &PluginOktaSyncSettings{
				SyncAccessLists: true,
				DefaultOwners:   []string{"owner1"},
			},
		},
	}

	validCreds := &PluginCredentialsV1{
		Credentials: &PluginCredentialsV1_StaticCredentialsRef{
			&PluginStaticCredentialsRef{
				Labels: map[string]string{
					"label1": "value1",
				},
			},
		},
	}

	testCases := []struct {
		name        string
		settings    *PluginSpecV1_Okta
		creds       *PluginCredentialsV1
		assertErr   require.ErrorAssertionFunc
		assertValue func(*testing.T, *PluginOktaSettings)
	}{
		{
			name:      "valid values are preserved",
			settings:  validSettings,
			creds:     validCreds,
			assertErr: require.NoError,
			assertValue: func(t *testing.T, settings *PluginOktaSettings) {
				require.Equal(t, "https://test.okta.com", settings.OrgUrl)
				require.True(t, settings.EnableUserSync)
				require.Equal(t, "some-sso-connector-id", settings.SsoConnectorId)
				require.True(t, settings.SyncSettings.SyncUsers)
				require.Equal(t, "some-sso-connector-id", settings.SyncSettings.SsoConnectorId)
				require.False(t, settings.SyncSettings.SyncAccessLists)
			},
		},
		{
			name:      "valid values are preserved, import populated",
			settings:  validSettingsWithSyncSettings,
			creds:     validCreds,
			assertErr: require.NoError,
			assertValue: func(t *testing.T, settings *PluginOktaSettings) {
				require.Equal(t, "https://test.okta.com", settings.OrgUrl)
				require.True(t, settings.EnableUserSync)
				require.False(t, settings.SyncSettings.SyncUsers) // Mismatch because there are sync settings.
				require.True(t, settings.SyncSettings.SyncAccessLists)
				require.ElementsMatch(t, []string{"owner1"}, settings.SyncSettings.DefaultOwners)
			},
		},
		{
			name: "no settings",
			settings: &PluginSpecV1_Okta{
				Okta: nil,
			},
			creds:     validCreds,
			assertErr: requireBadParameterWith("missing Okta settings"),
		},
		{
			name: "no org URL",
			settings: &PluginSpecV1_Okta{
				Okta: &PluginOktaSettings{},
			},
			creds:     validCreds,
			assertErr: requireBadParameterWith("org_url must be set"),
		},
		{
			name: "no credentials inner",
			settings: &PluginSpecV1_Okta{
				Okta: &PluginOktaSettings{
					OrgUrl: "https://test.okta.com",
				},
			},
			creds:     &PluginCredentialsV1{},
			assertErr: requireBadParameterWith("must be used with the static credentials ref type"),
		},
		{
			name: "invalid credential type (oauth2)",
			settings: &PluginSpecV1_Okta{
				Okta: &PluginOktaSettings{
					OrgUrl: "https://test.okta.com",
				},
			},
			creds: &PluginCredentialsV1{
				Credentials: &PluginCredentialsV1_Oauth2AccessToken{},
			},
			assertErr: requireBadParameterWith("must be used with the static credentials ref type"),
		},
		{
			name: "invalid credentials (static credentials)",
			settings: &PluginSpecV1_Okta{
				Okta: &PluginOktaSettings{
					OrgUrl: "https://test.okta.com",
				},
			},
			creds: &PluginCredentialsV1{
				Credentials: &PluginCredentialsV1_StaticCredentialsRef{
					&PluginStaticCredentialsRef{
						Labels: map[string]string{},
					},
				},
			},
			assertErr: requireBadParameterWith("labels must be specified"),
		}, {
			name: "EnableUserSync defaults to false",
			settings: &PluginSpecV1_Okta{
				Okta: &PluginOktaSettings{
					OrgUrl: "https://test.okta.com",
				},
			},
			creds:     validCreds,
			assertErr: require.NoError,
			assertValue: func(t *testing.T, settings *PluginOktaSettings) {
				require.False(t, settings.EnableUserSync)
			},
		}, {
			name: "SSO connector ID required for user sync",
			settings: &PluginSpecV1_Okta{
				Okta: &PluginOktaSettings{
					OrgUrl:         "https://test.okta.com",
					EnableUserSync: true,
				},
			},
			creds:     validCreds,
			assertErr: require.Error,
		}, {
			name: "SSO connector ID not required without user sync",
			settings: &PluginSpecV1_Okta{
				Okta: &PluginOktaSettings{
					OrgUrl:         "https://test.okta.com",
					EnableUserSync: false,
				},
			},
			creds:     validCreds,
			assertErr: require.NoError,
			assertValue: func(t *testing.T, settings *PluginOktaSettings) {
				require.False(t, settings.EnableUserSync)
				require.Empty(t, settings.SsoConnectorId)
				require.False(t, settings.SyncSettings.SyncUsers)
				require.Empty(t, settings.SyncSettings.SsoConnectorId)
			},
		}, {
			name: "import enabled without default owners",
			settings: &PluginSpecV1_Okta{
				Okta: &PluginOktaSettings{
					OrgUrl:         "https://test.okta.com",
					EnableUserSync: true,
					SsoConnectorId: "some-sso-connector-id",
					SyncSettings: &PluginOktaSyncSettings{
						SyncAccessLists: true,
					},
				},
			},
			creds:     validCreds,
			assertErr: requireBadParameterWith("default owners must be set when access list import is enabled"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			plugin := NewPluginV1(Metadata{Name: "foobar"}, PluginSpecV1{
				Settings: tc.settings,
			}, tc.creds)
			tc.assertErr(t, plugin.CheckAndSetDefaults())
			if tc.assertValue != nil {
				tc.assertValue(t, plugin.Spec.GetOkta())
			}
		})
	}
}

func TestPluginJamfValidation(t *testing.T) {
	testCases := []struct {
		name      string
		settings  *PluginSpecV1_Jamf
		creds     *PluginCredentialsV1
		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "no settings",
			settings: &PluginSpecV1_Jamf{
				Jamf: nil,
			},
			creds: nil,
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), "missing Jamf settings")
			},
		},
		{
			name: "no api Endpoint",
			settings: &PluginSpecV1_Jamf{
				Jamf: &PluginJamfSettings{
					JamfSpec: &JamfSpecV1{},
				},
			},
			creds: nil,
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), "api endpoint must be set")
			},
		},
		{
			name: "no credentials inner",
			settings: &PluginSpecV1_Jamf{
				Jamf: &PluginJamfSettings{
					JamfSpec: &JamfSpecV1{
						ApiEndpoint: "https://api.testjamfserver.com",
					},
				},
			},
			creds: &PluginCredentialsV1{},
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), "must be used with the static credentials ref type")
			},
		},
		{
			name: "invalid credential type (oauth2)",
			settings: &PluginSpecV1_Jamf{
				Jamf: &PluginJamfSettings{
					JamfSpec: &JamfSpecV1{
						ApiEndpoint: "https://api.testjamfserver.com",
					},
				},
			},
			creds: &PluginCredentialsV1{
				Credentials: &PluginCredentialsV1_Oauth2AccessToken{},
			},
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), "must be used with the static credentials ref type")
			},
		},
		{
			name: "invalid credentials (static credentials)",
			settings: &PluginSpecV1_Jamf{
				Jamf: &PluginJamfSettings{
					JamfSpec: &JamfSpecV1{
						ApiEndpoint: "https://api.testjamfserver.com",
					},
				},
			},
			creds: &PluginCredentialsV1{
				Credentials: &PluginCredentialsV1_StaticCredentialsRef{
					&PluginStaticCredentialsRef{
						Labels: map[string]string{},
					},
				},
			},
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), "labels must be specified")
			},
		},
		{
			name: "valid credentials (static credentials)",
			settings: &PluginSpecV1_Jamf{
				Jamf: &PluginJamfSettings{
					JamfSpec: &JamfSpecV1{
						ApiEndpoint: "https://api.testjamfserver.com",
					},
				},
			},
			creds: &PluginCredentialsV1{
				Credentials: &PluginCredentialsV1_StaticCredentialsRef{
					&PluginStaticCredentialsRef{
						Labels: map[string]string{
							"label1": "value1",
						},
					},
				},
			},
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			plugin := NewPluginV1(Metadata{Name: "foobar"}, PluginSpecV1{
				Settings: tc.settings,
			}, tc.creds)
			tc.assertErr(t, plugin.CheckAndSetDefaults())
		})
	}
}

func TestPluginMattermostValidation(t *testing.T) {
	defaultSettings := &PluginSpecV1_Mattermost{
		Mattermost: &PluginMattermostSettings{
			ServerUrl: "https://test.mattermost.com",
			Team:      "team-llama",
			Channel:   "teleport",
		},
	}

	testCases := []struct {
		name      string
		settings  *PluginSpecV1_Mattermost
		creds     *PluginCredentialsV1
		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "no settings",
			settings: &PluginSpecV1_Mattermost{
				Mattermost: nil,
			},
			creds: nil,
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), "missing Mattermost settings")
			},
		},
		{
			name: "no server url",
			settings: &PluginSpecV1_Mattermost{
				Mattermost: &PluginMattermostSettings{},
			},
			creds: nil,
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), "server url is required")
			},
		},
		{
			name: "no team",
			settings: &PluginSpecV1_Mattermost{
				Mattermost: &PluginMattermostSettings{
					ServerUrl: "https://test.mattermost.com",
					Channel:   "some-channel",
				},
			},
			creds: nil,
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), "team is required")
			},
		},
		{
			name: "no channel",
			settings: &PluginSpecV1_Mattermost{
				Mattermost: &PluginMattermostSettings{
					ServerUrl: "https://test.mattermost.com",
					Team:      "team-llama",
				},
			},
			creds: nil,
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), "channel is required")
			},
		},
		{
			name:     "no credentials inner",
			settings: defaultSettings,
			creds:    &PluginCredentialsV1{},
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), "must be used with the static credentials ref type")
			},
		},
		{
			name:     "invalid credential type (oauth2)",
			settings: defaultSettings,
			creds: &PluginCredentialsV1{
				Credentials: &PluginCredentialsV1_Oauth2AccessToken{},
			},
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), "must be used with the static credentials ref type")
			},
		},
		{
			name:     "no labels for credentials",
			settings: defaultSettings,
			creds: &PluginCredentialsV1{
				Credentials: &PluginCredentialsV1_StaticCredentialsRef{
					&PluginStaticCredentialsRef{
						Labels: map[string]string{},
					},
				},
			},
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.Contains(t, err.Error(), "labels must be specified")
			},
		},
		{
			name:     "valid settings with team/channel",
			settings: defaultSettings,
			creds: &PluginCredentialsV1{
				Credentials: &PluginCredentialsV1_StaticCredentialsRef{
					&PluginStaticCredentialsRef{
						Labels: map[string]string{
							"label1": "value1",
						},
					},
				},
			},
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.NoError(t, err)
			},
		},
		{
			name: "valid settings with no team/channel",
			settings: &PluginSpecV1_Mattermost{
				Mattermost: &PluginMattermostSettings{
					ServerUrl: "https://test.mattermost.com",
				},
			},
			creds: &PluginCredentialsV1{
				Credentials: &PluginCredentialsV1_StaticCredentialsRef{
					&PluginStaticCredentialsRef{
						Labels: map[string]string{
							"label1": "value1",
						},
					},
				},
			},
			assertErr: func(t require.TestingT, err error, args ...any) {
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			plugin := NewPluginV1(Metadata{Name: "foobar"}, PluginSpecV1{
				Settings: tc.settings,
			}, tc.creds)
			tc.assertErr(t, plugin.CheckAndSetDefaults())
		})
	}
}

func requireBadParameterError(t require.TestingT, err error, args ...any) {
	if tt, ok := t.(*testing.T); ok {
		tt.Helper()
	}
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), args...)
}

func reqireNamedBadParameterError(name string) require.ErrorAssertionFunc {
	return func(t require.TestingT, err error, args ...any) {
		if tt, ok := t.(*testing.T); ok {
			tt.Helper()
		}
		require.ErrorContains(t, err, name)
		require.True(t, trace.IsBadParameter(err))
	}
}

func TestPluginJiraValidation(t *testing.T) {
	validSettings := func() *PluginSpecV1_Jira {
		return &PluginSpecV1_Jira{
			&PluginJiraSettings{
				ServerUrl:  "https://example.com",
				ProjectKey: "PRJ",
				IssueType:  "Task",
			},
		}
	}
	validCreds := func() *PluginCredentialsV1 {
		return &PluginCredentialsV1{
			Credentials: &PluginCredentialsV1_StaticCredentialsRef{
				&PluginStaticCredentialsRef{
					Labels: map[string]string{
						"jira/address":   "https://jira.example.com",
						"jira/project":   "PRJ",
						"jira/issueType": "Task",
					},
				},
			},
		}
	}

	testCases := []struct {
		name           string
		mutateSettings func(*PluginSpecV1_Jira)
		mutateCreds    func(*PluginCredentialsV1)
		assertErr      require.ErrorAssertionFunc
	}{
		{
			name:      "Valid",
			assertErr: require.NoError,
		}, {
			name:           "Missing Settings",
			mutateSettings: func(s *PluginSpecV1_Jira) { s.Jira = nil },
			assertErr:      requireBadParameterError,
		}, {
			name:           "Missing Server URL",
			mutateSettings: func(s *PluginSpecV1_Jira) { s.Jira.ServerUrl = "" },
			assertErr:      reqireNamedBadParameterError("server URL"),
		}, {
			name:           "Missing Project Key",
			mutateSettings: func(s *PluginSpecV1_Jira) { s.Jira.ProjectKey = "" },
			assertErr:      reqireNamedBadParameterError("project key"),
		}, {
			name:           "Missing Issue Type",
			mutateSettings: func(s *PluginSpecV1_Jira) { s.Jira.IssueType = "" },
			assertErr:      reqireNamedBadParameterError("issue type"),
		}, {
			name:        "Missing Credentials",
			mutateCreds: func(c *PluginCredentialsV1) { c.Credentials = nil },
			assertErr:   requireBadParameterError,
		}, {
			name: "Missing Credential Labels",
			mutateCreds: func(c *PluginCredentialsV1) {
				c.Credentials.(*PluginCredentialsV1_StaticCredentialsRef).
					StaticCredentialsRef.
					Labels = map[string]string{}
			},
			assertErr: reqireNamedBadParameterError("labels"),
		}, {
			name: "Invalid Credential Type",
			mutateCreds: func(c *PluginCredentialsV1) {
				c.Credentials = &PluginCredentialsV1_Oauth2AccessToken{}
			},
			assertErr: reqireNamedBadParameterError("static credentials"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			settings := validSettings()
			if tc.mutateSettings != nil {
				tc.mutateSettings(settings)
			}

			creds := validCreds()
			if tc.mutateCreds != nil {
				tc.mutateCreds(creds)
			}

			plugin := NewPluginV1(Metadata{Name: "uut"}, PluginSpecV1{
				Settings: settings,
			}, creds)
			tc.assertErr(t, plugin.CheckAndSetDefaults())
		})
	}
}

func TestPluginDiscordValidation(t *testing.T) {
	validSettings := func() *PluginSpecV1_Discord {
		return &PluginSpecV1_Discord{
			&PluginDiscordSettings{
				RoleToRecipients: map[string]*DiscordChannels{
					"*": {ChannelIds: []string{"1234567890"}},
				},
			},
		}
	}
	validCreds := func() *PluginCredentialsV1 {
		return &PluginCredentialsV1{
			Credentials: &PluginCredentialsV1_StaticCredentialsRef{
				&PluginStaticCredentialsRef{
					Labels: map[string]string{},
				},
			},
		}
	}

	testCases := []struct {
		name           string
		mutateSettings func(*PluginSpecV1_Discord)
		mutateCreds    func(*PluginCredentialsV1)
		assertErr      require.ErrorAssertionFunc
	}{
		{
			name:      "Valid",
			assertErr: require.NoError,
		}, {
			name:           "Missing Settings",
			mutateSettings: func(s *PluginSpecV1_Discord) { s.Discord = nil },
			assertErr:      requireBadParameterError,
		}, {
			name: "Empty Role Mapping",
			mutateSettings: func(s *PluginSpecV1_Discord) {
				s.Discord.RoleToRecipients = map[string]*DiscordChannels{}
			},
			assertErr: reqireNamedBadParameterError("role_to_recipients"),
		}, {
			name: "Missing Default Mapping",
			mutateSettings: func(s *PluginSpecV1_Discord) {
				delete(s.Discord.RoleToRecipients, Wildcard)
				s.Discord.RoleToRecipients["access"] = &DiscordChannels{
					ChannelIds: []string{"1234567890"},
				}
			},
			assertErr: reqireNamedBadParameterError("default entry"),
		}, {
			name:        "Missing Credentials",
			mutateCreds: func(c *PluginCredentialsV1) { c.Credentials = nil },
			assertErr:   requireBadParameterError,
		}, {
			name: "Invalid Credential Type",
			mutateCreds: func(c *PluginCredentialsV1) {
				c.Credentials = &PluginCredentialsV1_Oauth2AccessToken{}
			},
			assertErr: reqireNamedBadParameterError("static credentials"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			settings := validSettings()
			if tc.mutateSettings != nil {
				tc.mutateSettings(settings)
			}

			creds := validCreds()
			if tc.mutateCreds != nil {
				tc.mutateCreds(creds)
			}

			plugin := NewPluginV1(
				Metadata{Name: "uut"},
				PluginSpecV1{Settings: settings},
				creds)
			tc.assertErr(t, plugin.CheckAndSetDefaults())
		})
	}
}
