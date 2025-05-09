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

// TestLocks tests that CRUD operations on lock resources are
// replicated from the backend to the cache.
func TestLocks(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.Lock]{
		newResource: func(name string) (types.Lock, error) {
			return types.NewLock(
				name,
				types.LockSpecV2{
					Target: types.LockTarget{
						Role: "target-role",
					},
				},
			)
		},
		create: p.accessS.UpsertLock,
		list: func(ctx context.Context) ([]types.Lock, error) {
			results, err := p.accessS.GetLocks(ctx, false)
			return results, err
		},
		cacheGet: p.cache.GetLock,
		cacheList: func(ctx context.Context) ([]types.Lock, error) {
			results, err := p.cache.GetLocks(ctx, false)
			return results, err
		},
		update:    p.accessS.UpsertLock,
		deleteAll: p.accessS.DeleteAllLocks,
	})
}
