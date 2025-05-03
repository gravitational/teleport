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

type networkingRestrictionIndex string

const networkingRestrictionNameIndex networkingRestrictionIndex = "name"

func newNetworkingRestrictionCollection(upstream services.Restrictions, w types.WatchKind) (*collection[types.NetworkRestrictions, networkingRestrictionIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter Restrictions")
	}

	return &collection[types.NetworkRestrictions, networkingRestrictionIndex]{
		store: newStore(
			types.NetworkRestrictions.Clone,
			map[networkingRestrictionIndex]func(types.NetworkRestrictions) string{
				networkingRestrictionNameIndex: types.NetworkRestrictions.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.NetworkRestrictions, error) {
			restrictions, err := upstream.GetNetworkRestrictions(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return []types.NetworkRestrictions{restrictions}, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) types.NetworkRestrictions {
			return &types.NetworkRestrictionsV4{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: types.Metadata{
					Name: hdr.Metadata.Name,
				},
			}
		},
		watch: w,
	}, nil
}

// GetNetworkRestrictions gets the network restrictions.
func (c *Cache) GetNetworkRestrictions(ctx context.Context) (types.NetworkRestrictions, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetNetworkRestrictions")
	defer span.End()

	getter := genericGetter[types.NetworkRestrictions, networkingRestrictionIndex]{
		cache:      c,
		collection: c.collections.networkRestrictions,
		index:      networkingRestrictionNameIndex,
		upstreamGet: func(ctx context.Context, s string) (types.NetworkRestrictions, error) {
			restriction, err := c.Config.Restrictions.GetNetworkRestrictions(ctx)
			return restriction, trace.Wrap(err)
		},
	}
	out, err := getter.get(ctx, types.MetaNameNetworkRestrictions)
	return out, trace.Wrap(err)
}
