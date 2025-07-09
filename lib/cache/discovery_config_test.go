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

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/header"
)

// TestDiscoveryConfig tests that CRUD operations on DiscoveryConfig resources are
// replicated from the backend to the cache.
func TestDiscoveryConfig(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[*discoveryconfig.DiscoveryConfig]{
		newResource: func(name string) (*discoveryconfig.DiscoveryConfig, error) {
			dc, err := discoveryconfig.NewDiscoveryConfig(
				header.Metadata{Name: "mydc"},
				discoveryconfig.Spec{
					DiscoveryGroup: "group001",
				})
			require.NoError(t, err)
			return dc, nil
		},
		create: func(ctx context.Context, discoveryConfig *discoveryconfig.DiscoveryConfig) error {
			_, err := p.discoveryConfigs.CreateDiscoveryConfig(ctx, discoveryConfig)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*discoveryconfig.DiscoveryConfig, error) {
			results, _, err := p.discoveryConfigs.ListDiscoveryConfigs(ctx, 0, "")
			return results, err
		},
		cacheGet: p.cache.GetDiscoveryConfig,
		cacheList: func(ctx context.Context) ([]*discoveryconfig.DiscoveryConfig, error) {
			results, _, err := p.cache.ListDiscoveryConfigs(ctx, 0, "")
			return results, err
		},
		update: func(ctx context.Context, discoveryConfig *discoveryconfig.DiscoveryConfig) error {
			_, err := p.discoveryConfigs.UpdateDiscoveryConfig(ctx, discoveryConfig)
			return trace.Wrap(err)
		},
		deleteAll: p.discoveryConfigs.DeleteAllDiscoveryConfigs,
	})
}
