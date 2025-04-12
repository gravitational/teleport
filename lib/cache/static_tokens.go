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

func newStaticTokensCollection(c services.ClusterConfiguration, w types.WatchKind) (*collection[types.StaticTokens], error) {
	if c == nil {
		return nil, trace.BadParameter("missing parameter ClusterConfig")
	}

	return &collection[types.StaticTokens]{
		store: newStore(map[string]func(types.StaticTokens) string{
			"name": func(u types.StaticTokens) string {
				return u.GetName()
			},
		}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.StaticTokens, error) {
			token, err := c.GetStaticTokens()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return []types.StaticTokens{token}, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) types.StaticTokens {
			return &types.StaticTokensV2{
				Kind:    types.KindStaticTokens,
				Version: types.V2,
				Metadata: types.Metadata{
					Name: types.MetaNameStaticTokens,
				},
			}
		},
		watch: w,
	}, nil

}

// GetStaticTokens gets the list of static tokens used to provision nodes.
func (c *Cache) GetStaticTokens() (types.StaticTokens, error) {
	_, span := c.Tracer.Start(context.TODO(), "cache/GetStaticTokens")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.staticTokens)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if rg.ReadCache() {
		st, err := rg.store.get("name", types.MetaNameStaticTokens)
		return st.Clone(), trace.Wrap(err)
	}

	st, err := c.Config.ClusterConfig.GetStaticTokens()
	return st, trace.Wrap(err)
}
