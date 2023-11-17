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

	"github.com/gravitational/teleport/api/types"
)

func TestMarshalPluginRoundTrip(t *testing.T) {
	spec := types.PluginSpecV1{
		Settings: &types.PluginSpecV1_SlackAccessPlugin{
			SlackAccessPlugin: &types.PluginSlackAccessSettings{
				FallbackChannel: "#access-requests",
			},
		},
	}

	creds := &types.PluginCredentialsV1{
		Credentials: &types.PluginCredentialsV1_Oauth2AccessToken{
			Oauth2AccessToken: &types.PluginOAuth2AccessTokenCredentials{
				AccessToken:  "access_token",
				RefreshToken: "refresh_token",
				Expires:      time.Now().UTC(),
			},
		},
	}

	plugin := types.NewPluginV1(types.Metadata{Name: "foobar"}, spec, creds)

	payload, err := MarshalPlugin(plugin)
	require.NoError(t, err)

	unmarshaled, err := UnmarshalPlugin(payload)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(plugin, unmarshaled))
}
