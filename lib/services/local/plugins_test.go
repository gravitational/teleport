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

package local

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
)

// TestPluginsCRUD tests backend operations with plugin resources.
func TestPluginsCRUD(t *testing.T) {
	ctx := context.Background()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { mem.Close() })

	service := NewPluginsService(mem)

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
	require.Empty(t, out)

	// Create both plugins.
	err = service.CreatePlugin(ctx, plugin1)
	require.NoError(t, err)
	err = service.CreatePlugin(ctx, plugin2)
	require.NoError(t, err)

	// Fetch all plugins.
	out, err = service.GetPlugins(ctx, true)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Plugin{plugin1, plugin2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Fetch a specific plugin.
	cluster, err := service.GetPlugin(ctx, plugin2.GetName(), true)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(plugin2, cluster,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Try to fetch a plugin that doesn't exist.
	_, err = service.GetPlugin(ctx, "doesnotexist", true)
	require.IsType(t, trace.NotFound(""), err)

	// Try to create a duplicate plugin.
	err = service.CreatePlugin(ctx, plugin1)
	require.IsType(t, trace.AlreadyExists(""), err)

	// Set plugin status.
	status := &types.PluginStatusV1{
		Code: types.PluginStatusCode_OTHER_ERROR,
	}
	err = service.SetPluginStatus(ctx, plugin1.GetName(), status)
	require.NoError(t, err)
	cluster, err = service.GetPlugin(ctx, plugin1.GetName(), true)
	require.NoError(t, err)
	// Fields other than Status should remain unchanged
	require.Empty(t, cmp.Diff(plugin1, cluster,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		cmpopts.IgnoreFields(types.PluginV1{}, "Status"),
	))
	require.Empty(t, cmp.Diff(status, cluster.GetStatus()))

	// Test if plugin types exist.
	exists, err := service.HasPluginType(ctx, types.PluginTypeOkta)
	require.NoError(t, err)
	require.False(t, exists)

	exists, err = service.HasPluginType(ctx, types.PluginTypeSlack)
	require.NoError(t, err)
	require.True(t, exists)

	// Delete a plugin.
	err = service.DeletePlugin(ctx, plugin1.GetName())
	require.NoError(t, err)
	out, err = service.GetPlugins(ctx, true)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Plugin{plugin2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Try to delete a plugin that doesn't exist.
	err = service.DeletePlugin(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Delete all plugin.
	err = service.DeleteAllPlugins(ctx)
	require.NoError(t, err)
	out, err = service.GetPlugins(ctx, true)
	require.NoError(t, err)
	require.Empty(t, out)
}

func TestListPlugins(t *testing.T) {
	const pageSize = 5
	const numPlugins = 2*pageSize + 1
	ctx := context.Background()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { mem.Close() })

	service := NewPluginsService(mem)

	var insertedPlugins []types.Plugin
	for i := 0; i < numPlugins; i++ {
		plugin := types.NewPluginV1(
			types.Metadata{Name: fmt.Sprintf("p%02d", i+1)},
			types.PluginSpecV1{
				Settings: &types.PluginSpecV1_SlackAccessPlugin{
					SlackAccessPlugin: &types.PluginSlackAccessSettings{
						FallbackChannel: fmt.Sprintf("#foo-%02d", i+1),
					},
				},
			},
			nil)
		err := service.CreatePlugin(ctx, plugin)
		require.NoError(t, err)
		insertedPlugins = append(insertedPlugins, plugin)
	}

	t.Run("paginated", func(t *testing.T) {
		page1, nextKey, err := service.ListPlugins(ctx, pageSize, "", true)
		require.NoError(t, err)
		require.NotEmpty(t, nextKey)
		require.Len(t, page1, pageSize)

		page2, nextKey, err := service.ListPlugins(ctx, pageSize, nextKey, true)
		require.NoError(t, err)
		require.NotEmpty(t, nextKey)
		require.Len(t, page2, pageSize)

		page3, nextKey, err := service.ListPlugins(ctx, pageSize, nextKey, true)
		require.NoError(t, err)
		require.Empty(t, nextKey)
		require.Len(t, page3, 1)

		var fetchedPlugins []types.Plugin
		fetchedPlugins = append(fetchedPlugins, page1...)
		fetchedPlugins = append(fetchedPlugins, page2...)
		fetchedPlugins = append(fetchedPlugins, page3...)

		require.Empty(t, cmp.Diff(insertedPlugins, fetchedPlugins,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		))
	})

	t.Run("single", func(t *testing.T) {
		fetchedPlugins, nextKey, err := service.ListPlugins(ctx, apidefaults.DefaultChunkSize, "", true)
		require.NoError(t, err)
		require.Empty(t, nextKey)

		require.Empty(t, cmp.Diff(insertedPlugins, fetchedPlugins,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		))
	})

	t.Run("zero page size uses default value", func(t *testing.T) {
		fetchedPlugins, nextKey, err := service.ListPlugins(ctx, 0, "", true)
		require.NoError(t, err)
		require.Empty(t, nextKey)

		require.Empty(t, cmp.Diff(insertedPlugins, fetchedPlugins,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		))
	})
}
