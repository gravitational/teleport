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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

// TestDiscoveryConfigCRUD tests backend operations with discovery config resources.
func TestDiscoveryConfigCRUD(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service, err := NewDiscoveryConfigService(backend.NewSanitizer(mem))
	require.NoError(t, err)

	// Create a couple discovery configs.
	discoveryConfig1 := newDiscoveryConfig(t, "discovery-config-1")
	discoveryConfig2 := newDiscoveryConfig(t, "discovery-config-2")
	discoveryConfig3 := newDiscoveryConfig(t, "discovery-config-3")

	// Initially we expect no discovery configs.
	out, nextToken, err := service.ListDiscoveryConfigs(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreFields(header.Metadata{}, "ID", "Revision"),
	}

	// Create both discovery configs.
	discoveryConfig, err := service.CreateDiscoveryConfig(ctx, discoveryConfig1)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(discoveryConfig1, discoveryConfig, cmpOpts...))
	discoveryConfig, err = service.CreateDiscoveryConfig(ctx, discoveryConfig2)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(discoveryConfig2, discoveryConfig, cmpOpts...))

	// Fetch a paginated list of discovery configs
	paginatedOut := make([]*discoveryconfig.DiscoveryConfig, 0, 2)
	for {
		out, nextToken, err = service.ListDiscoveryConfigs(ctx, 1, nextToken)
		require.NoError(t, err)

		paginatedOut = append(paginatedOut, out...)
		if nextToken == "" {
			break
		}
	}

	require.Len(t, paginatedOut, 2)
	require.Empty(t, cmp.Diff([]*discoveryconfig.DiscoveryConfig{discoveryConfig1, discoveryConfig2}, paginatedOut, cmpOpts...))

	// Fetch a specific discovery config.
	discoveryConfig, err = service.GetDiscoveryConfig(ctx, discoveryConfig2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(discoveryConfig2, discoveryConfig, cmpOpts...))

	// Try to fetch a discovery config that doesn't exist.
	_, err = service.GetDiscoveryConfig(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err), "expected not found error, got %v", err)

	// Update a discovery config.
	discoveryConfig1.SetExpiry(clock.Now().Add(30 * time.Minute))
	discoveryConfig, err = service.UpdateDiscoveryConfig(ctx, discoveryConfig1)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(discoveryConfig1, discoveryConfig, cmpOpts...))
	discoveryConfig, err = service.GetDiscoveryConfig(ctx, discoveryConfig1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(discoveryConfig1, discoveryConfig, cmpOpts...))

	// Upsert a discovery config updates if it already exists.
	discoveryConfig1.SetExpiry(clock.Now().Add(40 * time.Minute))
	discoveryConfig, err = service.UpsertDiscoveryConfig(ctx, discoveryConfig1)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(discoveryConfig1, discoveryConfig, cmpOpts...))
	discoveryConfig, err = service.GetDiscoveryConfig(ctx, discoveryConfig1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(discoveryConfig1, discoveryConfig, cmpOpts...))

	// Upsert a discovery config creates if it doesn't exist.
	discoveryConfig, err = service.UpsertDiscoveryConfig(ctx, discoveryConfig3)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(discoveryConfig3, discoveryConfig, cmpOpts...))

	// Delete a discovery config.
	err = service.DeleteDiscoveryConfig(ctx, discoveryConfig1.GetName())
	require.NoError(t, err)
	out, nextToken, err = service.ListDiscoveryConfigs(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]*discoveryconfig.DiscoveryConfig{discoveryConfig2, discoveryConfig3}, out, cmpOpts...))

	// Try to delete a discovery config that doesn't exist.
	err = service.DeleteDiscoveryConfig(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err), "expected not found error, got %v", err)

	// Delete all discovery configs.
	err = service.DeleteAllDiscoveryConfigs(ctx)
	require.NoError(t, err)
	out, nextToken, err = service.ListDiscoveryConfigs(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)
}

func newDiscoveryConfig(t *testing.T, name string) *discoveryconfig.DiscoveryConfig {
	t.Helper()

	discoveryConfig, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{
			Name: name,
		},
		discoveryconfig.Spec{
			DiscoveryGroup: "dg-1",
		},
	)
	require.NoError(t, err)

	return discoveryConfig
}
