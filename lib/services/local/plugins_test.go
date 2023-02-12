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

package local

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
)

// TestPluginsCRUD tests backend operations with plugin resources.
func TestPluginsCRUD(t *testing.T) {
	ctx := context.Background()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service := NewPluginsService(backend)

	// Define two plugins
	plugin1 := types.NewPluginV1(types.Metadata{Name: "p1"}, types.PluginSpecV1{
		Settings: &types.PluginSpecV1_SlackAccessPlugin{
			SlackAccessPlugin: &types.PluginSlackAccessSettings{
				FallbackChannel: "#foo",
			},
		},
	}, nil)
	plugin2 := types.NewPluginV1(types.Metadata{Name: "p2"}, types.PluginSpecV1{
		Settings: &types.PluginSpecV1_SlackAccessPlugin{
			SlackAccessPlugin: &types.PluginSlackAccessSettings{
				FallbackChannel: "#bar",
			},
		},
	}, nil)

	// Initially we expect no items.
	out, err := service.GetPlugins(ctx, false)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))

	// Create both plugins.
	err = service.CreatePlugin(ctx, plugin1)
	require.NoError(t, err)
	err = service.CreatePlugin(ctx, plugin2)
	require.NoError(t, err)

	// Fetch all plugins.
	out, err = service.GetPlugins(ctx, true)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Plugin{plugin1, plugin2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Fetch a specific plugin.
	cluster, err := service.GetPlugin(ctx, plugin2.GetName(), true)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(plugin2, cluster,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Try to fetch a plugin that doesn't exist.
	_, err = service.GetPlugin(ctx, "doesnotexist", true)
	require.IsType(t, trace.NotFound(""), err)

	// Try to create a duplicate plugin.
	err = service.CreatePlugin(ctx, plugin1)
	require.IsType(t, trace.AlreadyExists(""), err)

	// Set plugin status.
	status := types.PluginStatusV1{
		Code: types.PluginStatusCode_OTHER_ERROR,
	}
	err = service.SetPluginStatus(ctx, plugin1.GetName(), status)
	require.NoError(t, err)
	cluster, err = service.GetPlugin(ctx, plugin1.GetName(), true)
	require.NoError(t, err)
	// Fields other than Status should remain unchanged
	require.Empty(t, cmp.Diff(plugin1, cluster,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
		cmpopts.IgnoreFields(types.PluginV1{}, "Status"),
	))
	require.Empty(t, cmp.Diff(status, cluster.GetStatus()))

	// Delete a plugin.
	err = service.DeletePlugin(ctx, plugin1.GetName())
	require.NoError(t, err)
	out, err = service.GetPlugins(ctx, true)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Plugin{plugin2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Try to delete a plugin that doesn't exist.
	err = service.DeletePlugin(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Delete all plugin.
	err = service.DeleteAllPlugins(ctx)
	require.NoError(t, err)
	out, err = service.GetPlugins(ctx, true)
	require.NoError(t, err)
	require.Len(t, out, 0)
}
