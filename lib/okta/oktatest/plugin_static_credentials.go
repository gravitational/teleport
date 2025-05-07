/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package oktatest

import (
	"context"
	"testing" //nolint:depguard // this a shared test package

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require" //nolint:depguard // this a shared test package

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type pluginStaticCredentialsOption func(*types.PluginStaticCredentialsSpecV1)

func WithOAuthClientID(clientID string) pluginStaticCredentialsOption {
	return func(spec *types.PluginStaticCredentialsSpecV1) {
		spec.Credentials = &types.PluginStaticCredentialsSpecV1_OAuthClientSecret{
			OAuthClientSecret: &types.PluginStaticCredentialsOAuthClientSecret{
				ClientId:     clientID,
				ClientSecret: "",
			},
		}
	}
}

func WithAPIToken(apiToken string) pluginStaticCredentialsOption {
	return func(spec *types.PluginStaticCredentialsSpecV1) {
		spec.Credentials = &types.PluginStaticCredentialsSpecV1_APIToken{
			APIToken: apiToken,
		}
	}
}

func NewPluginStaticCredentials(t *testing.T, plugin *types.PluginV1, opts ...pluginStaticCredentialsOption) types.PluginStaticCredentials {
	t.Helper()

	require.NotNil(t, plugin.GetCredentials().GetStaticCredentialsRef(), "plugin credentials ref missing")
	require.NotEmpty(t, plugin.GetCredentials().GetStaticCredentialsRef().Labels, "plugin credentials ref labels missing")

	spec := types.PluginStaticCredentialsSpecV1{
		Credentials: &types.PluginStaticCredentialsSpecV1_OAuthClientSecret{
			OAuthClientSecret: &types.PluginStaticCredentialsOAuthClientSecret{
				ClientId:     "test-client-id-" + uuid.NewString(),
				ClientSecret: "",
			},
		},
	}

	for _, opt := range opts {
		opt(&spec)
	}

	staticCreds, err := types.NewPluginStaticCredentials(
		types.Metadata{
			Name:   types.PluginTypeOkta,
			Labels: plugin.GetCredentials().GetStaticCredentialsRef().Labels,
		},
		spec,
	)
	require.NoError(t, err, "types.NewPluginStaticCredentials")
	return staticCreds
}

// UpsertPluginStaticCredentials upserts Okta plugin ignoring the revision.
func UpsertPluginStaticCredentials(t *testing.T, credsSvc services.PluginStaticCredentials, staticCreds types.PluginStaticCredentials) {
	t.Helper()
	ctx := context.Background()

	require.Equal(t, types.PluginTypeOkta, staticCreds.GetName())

	old, err := credsSvc.GetPluginStaticCredentials(ctx, staticCreds.GetName())
	if trace.IsNotFound(err) {
		err := credsSvc.CreatePluginStaticCredentials(ctx, staticCreds)
		require.NoError(t, err, "creating test Okta plugin static credentials")
		return
	} else {
		require.NoError(t, err, "getting test Okta plugin static credentials")
	}

	staticCreds.SetRevision(old.GetRevision())
	_, err = credsSvc.UpdatePluginStaticCredentials(ctx, staticCreds)
	require.NoError(t, err, "updating test Okta plugin static credentials")
}
