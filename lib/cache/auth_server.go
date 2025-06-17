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

type authServerIndex string

const authServerNameIndex authServerIndex = "name"

func newAuthServerCollection(p services.Presence, w types.WatchKind) (*collection[types.Server, authServerIndex], error) {
	if p == nil {
		return nil, trace.BadParameter("missing parameter Presence")
	}

	return &collection[types.Server, authServerIndex]{
		store: newStore(
			types.Server.DeepCopy,
			map[authServerIndex]func(types.Server) string{
				authServerNameIndex: types.Server.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.Server, error) {
			servers, err := p.GetAuthServers()
			return servers, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.Server {
			return &types.ServerV2{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: types.Metadata{
					Name: hdr.GetName(),
				},
			}
		},
		watch: w,
	}, nil
}

// GetAuthServers returns a list of registered servers
func (c *Cache) GetAuthServers() ([]types.Server, error) {
	_, span := c.Tracer.Start(context.TODO(), "cache/GetAuthServers")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.authServers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		servers, err := c.Config.Presence.GetAuthServers()
		return servers, trace.Wrap(err)
	}

	servers := make([]types.Server, 0, rg.store.len())
	for s := range rg.store.resources(authServerNameIndex, "", "") {
		servers = append(servers, s.DeepCopy())
	}

	return servers, nil
}
