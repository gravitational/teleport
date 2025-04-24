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

	"github.com/gravitational/teleport/api/types"
)

// TestProxies tests proxies cache
func TestProxies(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.Server]{
		newResource: func(name string) (types.Server, error) {
			return &types.ServerV2{
				Kind:    types.KindProxy,
				Version: types.V2,
				Metadata: types.Metadata{
					Name: name,
				},
				Spec: types.ServerSpecV2{
					Addr: "127.0.0.1:2022",
				},
			}, nil
		},
		create: p.presenceS.UpsertProxy,
		list: func(_ context.Context) ([]types.Server, error) {
			return p.presenceS.GetProxies()
		},
		cacheList: func(_ context.Context) ([]types.Server, error) {
			return p.cache.GetProxies()
		},
		update: p.presenceS.UpsertProxy,
		deleteAll: func(_ context.Context) error {
			return p.presenceS.DeleteAllProxies()
		},
	})
}
