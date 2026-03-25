// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func newPlugin(name string) *types.PluginV1 {
	return &types.PluginV1{
		Metadata: types.Metadata{Name: name},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Scim{
				Scim: &types.PluginSCIMSettings{
					SamlConnectorName: "example-saml-connector",
				},
			},
		},
	}
}

func newPluginWithCreds(name string) *types.PluginV1 {
	item := newPlugin(name)
	creds := types.PluginCredentialsV1{
		Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
			StaticCredentialsRef: &types.PluginStaticCredentialsRef{
				Labels: map[string]string{
					"env": "prod",
				},
			},
		},
	}
	item.SetCredentials(&creds)
	return item
}

// TestPlugin tests caching of plugins
func TestPlugin(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	t.Run("GetPlugins", func(t *testing.T) {
		testResources(t, p, testFuncs[types.Plugin]{
			newResource: func(name string) (types.Plugin, error) {
				return newPlugin(name), nil
			},
			create: func(ctx context.Context, item types.Plugin) error {
				err := p.plugin.CreatePlugin(ctx, item)
				return err
			},
			cacheGet: func(ctx context.Context, name string) (types.Plugin, error) {
				return p.cache.GetPlugin(ctx, name, false)
			},
			list: func(ctx context.Context, pageSize int, pageToken string) ([]types.Plugin, string, error) {
				return p.plugin.ListPlugins(ctx, pageSize, pageToken, false)
			},
			cacheList: func(ctx context.Context, pageSize int, pageToken string) ([]types.Plugin, string, error) {
				return p.cache.ListPlugins(ctx, pageSize, pageToken, false)
			},
			update: func(ctx context.Context, item types.Plugin) error {
				_, err := p.plugin.UpdatePlugin(ctx, item)
				return err
			},
			deleteAll: func(ctx context.Context) error {
				return p.plugin.DeleteAllPlugins(ctx)
			},
		})
	})

	t.Run("ListPlugins", func(t *testing.T) {
		testResources(t, p, testFuncs[types.Plugin]{
			newResource: func(name string) (types.Plugin, error) {
				return newPlugin(name), nil
			},
			create: func(ctx context.Context, item types.Plugin) error {
				return trace.Wrap(p.plugin.CreatePlugin(ctx, item))
			},
			list: func(ctx context.Context, pageSize int, token string) ([]types.Plugin, string, error) {
				return p.plugin.ListPlugins(ctx, pageSize, token, false)
			},
			cacheGet: func(ctx context.Context, name string) (types.Plugin, error) {
				return p.cache.GetPlugin(ctx, name, false)
			},
			cacheList: func(ctx context.Context, pageSize int, token string) ([]types.Plugin, string, error) {
				return p.cache.ListPlugins(ctx, pageSize, token, false)
			},
			update: func(ctx context.Context, item types.Plugin) error {
				_, err := p.plugin.UpdatePlugin(ctx, item)
				return err
			},
			deleteAll: func(ctx context.Context) error {
				return p.plugin.DeleteAllPlugins(ctx)
			},
		})
	})
	t.Run("GetPluginsWithSecrets", func(t *testing.T) {
		testResources(t, p, testFuncs[types.Plugin]{
			newResource: func(name string) (types.Plugin, error) {
				return newPluginWithCreds(name), nil
			},
			create: func(ctx context.Context, item types.Plugin) error {
				err := p.plugin.CreatePlugin(ctx, item)
				return err
			},
			cacheGet: func(ctx context.Context, name string) (types.Plugin, error) {
				return p.cache.GetPlugin(ctx, name, true)
			},
			list: func(ctx context.Context, pageSize int, token string) ([]types.Plugin, string, error) {
				return p.plugin.ListPlugins(ctx, pageSize, token, true)
			},
			cacheList: func(ctx context.Context, pageSize int, token string) ([]types.Plugin, string, error) {
				return p.cache.ListPlugins(ctx, pageSize, token, true)
			},
			update: func(ctx context.Context, item types.Plugin) error {
				_, err := p.plugin.UpdatePlugin(ctx, item)
				return err
			},
			deleteAll: func(ctx context.Context) error {
				return p.plugin.DeleteAllPlugins(ctx)
			},
		})
	})
}

func TestPlugin_HasPluginType(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	slackPlugin := newPlugin("test_slack_1")
	slackPlugin.Spec.Settings = &types.PluginSpecV1_SlackAccessPlugin{
		SlackAccessPlugin: &types.PluginSlackAccessSettings{
			FallbackChannel: "#foo",
		},
	}
	scimPlugin := newPlugin("test_scim_1")
	scimPlugin.Spec.Settings = &types.PluginSpecV1_Scim{
		Scim: &types.PluginSCIMSettings{
			SamlConnectorName: "example-saml-connector",
		},
	}

	err := p.plugin.CreatePlugin(ctx, slackPlugin)
	require.NoError(t, err)

	err = p.plugin.CreatePlugin(ctx, scimPlugin)
	require.NoError(t, err)

	// Wait for cache propagation.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		plugins, _, err := p.cache.ListPlugins(ctx, 0, "", false)
		require.NoError(t, err)
		require.Len(t, plugins, 2)
	}, 15*time.Second, 100*time.Millisecond)

	has, err := p.cache.HasPluginType(ctx, types.PluginTypeSlack)
	require.NoError(t, err)
	require.True(t, has)

	has, err = p.cache.HasPluginType(ctx, types.PluginTypeSCIM)
	require.NoError(t, err)
	require.True(t, has)

	has, err = p.cache.HasPluginType(ctx, types.PluginTypeOkta)
	require.NoError(t, err)
	require.False(t, has)
}
