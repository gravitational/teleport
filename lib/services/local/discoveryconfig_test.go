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

	// Initially we expect no discovery configs.
	out, nextToken, err := service.ListDiscoveryConfigs(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreFields(header.Metadata{}, "ID"),
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

	// Delete a discovery config.
	err = service.DeleteDiscoveryConfig(ctx, discoveryConfig1.GetName())
	require.NoError(t, err)
	out, nextToken, err = service.ListDiscoveryConfigs(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]*discoveryconfig.DiscoveryConfig{discoveryConfig2}, out, cmpOpts...))

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
