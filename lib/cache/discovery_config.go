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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/services"
)

type discoveryConfigIndex string

const discoveryConfigNameIndex discoveryConfigIndex = "name"

func newDiscoveryConfigCollection(upstream services.DiscoveryConfigs, w types.WatchKind) (*collection[*discoveryconfig.DiscoveryConfig, discoveryConfigIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter DiscoveryConfigs")
	}

	return &collection[*discoveryconfig.DiscoveryConfig, discoveryConfigIndex]{
		store: newStore(
			(*discoveryconfig.DiscoveryConfig).Clone,
			map[discoveryConfigIndex]func(*discoveryconfig.DiscoveryConfig) string{
				discoveryConfigNameIndex: func(r *discoveryconfig.DiscoveryConfig) string {
					return r.GetMetadata().Name
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*discoveryconfig.DiscoveryConfig, error) {
			var discoveryConfigs []*discoveryconfig.DiscoveryConfig
			var nextToken string
			for {
				var page []*discoveryconfig.DiscoveryConfig
				var err error

				page, nextToken, err = upstream.ListDiscoveryConfigs(ctx, 0 /* default page size */, nextToken)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				discoveryConfigs = append(discoveryConfigs, page...)

				if nextToken == "" {
					break
				}
			}
			return discoveryConfigs, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) *discoveryconfig.DiscoveryConfig {
			return &discoveryconfig.DiscoveryConfig{
				ResourceHeader: header.ResourceHeader{
					Kind:    hdr.Kind,
					Version: hdr.Version,
					Metadata: header.Metadata{
						Name: hdr.Metadata.Name,
					},
				},
			}
		},
		watch: w,
	}, nil
}

// ListDiscoveryConfigs returns a paginated list of all DiscoveryConfig resources.
func (c *Cache) ListDiscoveryConfigs(ctx context.Context, pageSize int, pageToken string) ([]*discoveryconfig.DiscoveryConfig, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListDiscoveryConfigs")
	defer span.End()

	lister := genericLister[*discoveryconfig.DiscoveryConfig, discoveryConfigIndex]{
		cache:        c,
		collection:   c.collections.discoveryConfigs,
		index:        discoveryConfigNameIndex,
		upstreamList: c.Config.DiscoveryConfigs.ListDiscoveryConfigs,
		nextToken: func(t *discoveryconfig.DiscoveryConfig) string {
			return t.GetMetadata().Name
		},
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

// GetDiscoveryConfig returns the specified DiscoveryConfig resource.
func (c *Cache) GetDiscoveryConfig(ctx context.Context, name string) (*discoveryconfig.DiscoveryConfig, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetDiscoveryConfig")
	defer span.End()

	getter := genericGetter[*discoveryconfig.DiscoveryConfig, discoveryConfigIndex]{
		cache:       c,
		collection:  c.collections.discoveryConfigs,
		index:       discoveryConfigNameIndex,
		upstreamGet: c.Config.DiscoveryConfigs.GetDiscoveryConfig,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}
