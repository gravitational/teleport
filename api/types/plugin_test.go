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
