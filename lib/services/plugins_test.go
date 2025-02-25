/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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

func TestUnmarshalPluginUnknownField(t *testing.T) {
	t.Run("unknown field in plugin spec", func(t *testing.T) {
		payload := `
		{
		  "kind": "plugin",
		  "version": "v1",
		  "metadata": { "Name": "plugin", "Namespace": "default" },
		  "spec": { "slackAccessPlugin": { "fallbackChannel": "#access-requests", "unknownField": "val" }},
		  "credentials": { "oauth2AccessToken": {"accessToken": "token", "refreshToken": "token" }}
		  }
		}`
		plugin, err := UnmarshalPlugin([]byte(payload))
		require.NoError(t, err)
		require.Equal(t, "plugin", plugin.GetName())
		require.Equal(t, types.PluginType(types.PluginTypeSlack), plugin.GetType())
	})

	t.Run("unknown plugin type", func(t *testing.T) {
		payload := `
		{
		  "kind": "plugin",
		  "version": "v1",
		  "metadata": { "Name": "plugin", "Namespace": "default" },
		  "spec": { "unknownPlugin": { }},
		  "credentials": { "oauth2AccessToken": {"accessToken": "token", "refreshToken": "token" }}
		  }
		}`
		plugin, err := UnmarshalPlugin([]byte(payload))
		require.NoError(t, err)
		require.Equal(t, "plugin", plugin.GetName())
		require.Equal(t, types.PluginTypeUnknown, plugin.GetType())
	})
}

func TestMarshalPluginWithStatus(t *testing.T) {
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

	ts := time.Now()

	plugin := types.NewPluginV1(types.Metadata{Name: "foobar"}, spec, creds)
	status := &types.PluginStatusV1{
		Code: types.PluginStatusCode_RUNNING,
		Details: &types.PluginStatusV1_Okta{
			Okta: &types.PluginOktaStatusV1{
				UsersSyncDetails: &types.PluginOktaStatusDetailsUsersSync{
					Enabled:        true,
					LastSuccessful: &ts,
				},
				AccessListsSyncDetails: &types.PluginOktaStatusDetailsAccessListsSync{
					Enabled:         true,
					StatusCode:      types.OktaPluginSyncStatusCode_OKTA_PLUGIN_SYNC_STATUS_CODE_SUCCESS,
					LastSuccessful:  &ts,
					NumAppsSynced:   int32(42),
					NumGroupsSynced: int32(314159),
				},
			},
		},
	}
	require.NoError(t, plugin.SetStatus(status))

	payload, err := MarshalPlugin(plugin)
	require.NoError(t, err)

	unmarshaled, err := UnmarshalPlugin(payload)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(plugin, unmarshaled))
}
