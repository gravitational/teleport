/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	kubeprovisionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubeprovision/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
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

// TestKubeProvisionCRUD tests backend operations with KubeProvision resources.
func TestKubeProvisionCRUD(t *testing.T) {
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
		cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
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

func TestCreateKubeProvision(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getKubeProvisionService(t)

	kubeProvision := newKubeProvision(t, "provision1", &kubeprovisionv1.KubeProvisionSpec{})
	kubeProvisionOrig := newKubeProvision(t, "provision1", &kubeprovisionv1.KubeProvisionSpec{})

	// First attempt should succeed
	created, err := service.CreateKubeProvision(ctx, kubeProvision)
	require.NoError(t, err)

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreFields(headerv1.Metadata{}, "Revision"),
		cmpopts.IgnoreUnexported(kubeprovisionv1.KubeProvision{}, kubeprovisionv1.KubeProvisionSpec{}, headerv1.Metadata{}),
	}
	require.Empty(t, cmp.Diff(kubeProvisionOrig, created, cmpOpts...))

	// Second attempt should fail, kubeProvision already exists
	_, err = service.CreateKubeProvision(ctx, kubeProvision)
	require.Error(t, err)
}

func newKubeProvision(t *testing.T, name string, spec *kubeprovisionv1.KubeProvisionSpec) *kubeprovisionv1.KubeProvision {
	t.Helper()

	kubeProvision := kubeprovisionv1.KubeProvision{
		Kind:    types.KindKubeProvision,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: spec,
	}

	return &kubeProvision
}

func getKubeProvisionService(t *testing.T) services.KubeProvisions {
	t.Helper()
	memoryBackend, err := memory.New(memory.Config{
		Context: context.Background(),
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service, err := NewKubeProvisionService(memoryBackend)
	require.NoError(t, err)
	return service
}
