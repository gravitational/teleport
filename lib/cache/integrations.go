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
	"github.com/gravitational/teleport/lib/services"
)

type integrationIndex string

const integrationNameIndex integrationIndex = "name"

func newIntegrationCollection(upstream services.Integrations, w types.WatchKind) (*collection[types.Integration, integrationIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter Integrations")
	}

	return &collection[types.Integration, integrationIndex]{
		store: newStore(
			types.Integration.Clone,
			map[integrationIndex]func(types.Integration) string{
				integrationNameIndex: types.Integration.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.Integration, error) {
			var startKey string
			var resources []types.Integration
			for {
				var igs []types.Integration
				var err error
				igs, startKey, err = upstream.ListIntegrations(ctx, 0, startKey)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				resources = append(resources, igs...)

				if startKey == "" {
					break
				}
			}

			return resources, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) types.Integration {
			return &types.IntegrationV1{
				ResourceHeader: types.ResourceHeader{
					Kind:    hdr.Kind,
					Version: hdr.Version,
					Metadata: types.Metadata{
						Name: hdr.Metadata.Name,
					},
				},
			}
		},
		watch: w,
	}, nil
}

// ListIntegrations returns a paginated list of all Integrations resources.
func (c *Cache) ListIntegrations(ctx context.Context, pageSize int, pageToken string) ([]types.Integration, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListIntegrations")
	defer span.End()

	lister := genericLister[types.Integration, integrationIndex]{
		cache:        c,
		collection:   c.collections.integrations,
		index:        integrationNameIndex,
		upstreamList: c.Config.Integrations.ListIntegrations,
		nextToken: func(t types.Integration) string {
			return t.GetMetadata().Name
		},
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

// GetIntegration returns the specified Integration resources.
func (c *Cache) GetIntegration(ctx context.Context, name string) (types.Integration, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetIntegration")
	defer span.End()

	getter := genericGetter[types.Integration, integrationIndex]{
		cache:       c,
		collection:  c.collections.integrations,
		index:       integrationNameIndex,
		upstreamGet: c.Config.Integrations.GetIntegration,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}
