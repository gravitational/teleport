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

	"github.com/gravitational/teleport/api/types"
)

func TestNetworkRestrictions(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.NetworkRestrictions]{
		newResource: func(name string) (types.NetworkRestrictions, error) {
			return types.NewNetworkRestrictions(), nil
		},
		create: p.restrictions.SetNetworkRestrictions,
		list: func(ctx context.Context) ([]types.NetworkRestrictions, error) {
			restrictions, err := p.restrictions.GetNetworkRestrictions(ctx)
			return []types.NetworkRestrictions{restrictions}, trace.Wrap(err)
		},
		cacheList: func(ctx context.Context) ([]types.NetworkRestrictions, error) {
			restrictions, err := p.cache.GetNetworkRestrictions(ctx)
			if trace.IsNotFound(err) {
				return nil, nil
			}
			return []types.NetworkRestrictions{restrictions}, trace.Wrap(err)
		},
		deleteAll: p.restrictions.DeleteNetworkRestrictions,
	})
}
