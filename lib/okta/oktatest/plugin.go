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
	oktaplugin "github.com/gravitational/teleport/lib/okta/plugin"
	"github.com/gravitational/teleport/lib/services"
)

type pluginOption func(*pluginOptions)

type pluginOptions struct {
	orgURL       string
	syncSettings *types.PluginOktaSyncSettings
}

func WithOrgURL(orgURL string) pluginOption {
	return func(pluginOpts *pluginOptions) {
		pluginOpts.orgURL = orgURL
	}
}

func WithSyncSettings(syncSettings *types.PluginOktaSyncSettings) pluginOption {
	return func(pluginOpts *pluginOptions) {
		pluginOpts.syncSettings = syncSettings
	}
}

// CreatePlugin creates a Okta plugin for test.
func NewPlugin(t *testing.T, opts ...pluginOption) *types.PluginV1 {
	t.Helper()

	pluginOpts := &pluginOptions{
		orgURL: "https://okta.example.com",
	}
	for _, opt := range opts {
		opt(pluginOpts)
	}

	plugin := types.NewPluginV1(
		types.Metadata{
			Name: types.PluginTypeOkta,
		},
		types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Okta{
				Okta: &types.PluginOktaSettings{
					OrgUrl:       pluginOpts.orgURL,
					SyncSettings: pluginOpts.syncSettings,
				},
			},
		},
		&types.PluginCredentialsV1{
			Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
				StaticCredentialsRef: &types.PluginStaticCredentialsRef{
					Labels: map[string]string{"test-cred-key": "test-cred-value-" + uuid.NewString()},
				},
			},
		},
	)

	require.NoError(t, plugin.CheckAndSetDefaults(), "validating test Okta plugin")
	return plugin
}

// UpsertPlugin upserts Okta plugin ignoring the revision.
func UpsertPlugin(t *testing.T, plugins services.Plugins, plugin *types.PluginV1) {
	t.Helper()
	ctx := context.Background()

	require.Equal(t, types.PluginTypeOkta, plugin.GetName())

	old, err := oktaplugin.Get(ctx, plugins, true /* withSecrets */)
	if trace.IsNotFound(err) {
		err := plugins.CreatePlugin(ctx, plugin)
		require.NoError(t, err, "creating test Okta plugin")
		return
	} else {
		require.NoError(t, err, "getting test Okta plugin")
	}

	plugin.SetRevision(old.GetRevision())
	_, err = plugins.UpdatePlugin(ctx, plugin)
	require.NoError(t, err, "updating test Okta plugin")
}
