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

package services

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
)

func requireIdenticalPlugins(t *testing.T, original, unmarshaled *types.PluginV1) {
	require.Empty(t, cmp.Diff(original, unmarshaled))
}

func TestMarshalPluginRoundTrip(t *testing.T) {

	staticCreds := &types.PluginCredentialsV1{
		Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
			StaticCredentialsRef: &types.PluginStaticCredentialsRef{
				Labels: map[string]string{"label": "value"},
			},
		},
	}

	testCases := []struct {
		name    string
		plugin  string
		spec    *types.PluginSpecV1
		creds   *types.PluginCredentialsV1
		marshal func(plugin types.Plugin, opts ...MarshalOption) ([]byte, error)
		assert  func(t *testing.T, original, marshaled *types.PluginV1)
	}{
		{
			name:   "slack",
			plugin: types.PluginTypeSlack,
			spec: &types.PluginSpecV1{
				Settings: &types.PluginSpecV1_SlackAccessPlugin{
					SlackAccessPlugin: &types.PluginSlackAccessSettings{
						FallbackChannel: "#access-requests",
					},
				},
			},
			creds: &types.PluginCredentialsV1{
				Credentials: &types.PluginCredentialsV1_Oauth2AccessToken{
					Oauth2AccessToken: &types.PluginOAuth2AccessTokenCredentials{
						AccessToken:  "access_token",
						RefreshToken: "refresh_token",
						Expires:      time.Now().UTC(),
					},
				},
			},
			marshal: MarshalPlugin,
			assert:  requireIdenticalPlugins,
		}, {
			name:   "okta v2",
			plugin: types.PluginTypeOkta,
			spec: &types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Okta{
					Okta: &types.PluginOktaSettings{
						Version: types.V2,
						OrgUrl:  "https://tenant.example.com",
					},
				},
			},
			creds:   staticCreds,
			marshal: MarshalPlugin,
			assert:  requireIdenticalPlugins,
		}, {
			name:   "unversioned okta adds version string on load",
			plugin: types.PluginTypeOkta,
			spec: &types.PluginSpecV1{
				Settings: &types.PluginSpecV1_Okta{
					Okta: &types.PluginOktaSettings{
						OrgUrl: "https://tenant.example.com",
					},
				},
			},
			creds:   staticCreds,
			marshal: marshalRawPlugin,
			assert: func(t *testing.T, original, marshaled *types.PluginV1) {
				s := marshaled.Spec.Settings.(*types.PluginSpecV1_Okta).Okta
				require.Equal(t, types.V1, s.Version)
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			plugin := types.NewPluginV1(
				types.Metadata{Name: test.plugin, Namespace: defaults.Namespace},
				*test.spec,
				test.creds)

			// Given a plugin record in JSON...
			payload, err := test.marshal(plugin)
			require.NoError(t, err)

			// When I unmarshal it into an in-memory record...
			unmarshaled, err := UnmarshalPlugin(payload)

			// The operation succeeds, and any specific content expectations
			// for the test are met
			require.NoError(t, err)
			test.assert(t, plugin, unmarshaled.(*types.PluginV1))
		})
	}
}
