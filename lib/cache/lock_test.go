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
	"iter"
	"testing"

	"github.com/gravitational/teleport/api/types"
)

// TestLocks tests that CRUD operations on lock resources are
// replicated from the backend to the cache.
func TestLocks(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	t.Run("GetLocks", func(t *testing.T) {
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
			create:    p.accessS.UpsertLock,
			list:      getAllAdapter(func(ctx context.Context) ([]types.Lock, error) { return p.accessS.GetLocks(ctx, false) }),
			cacheList: getAllAdapter(func(ctx context.Context) ([]types.Lock, error) { return p.cache.GetLocks(ctx, false) }),
			cacheGet:  p.cache.GetLock,
			update:    p.accessS.UpsertLock,
			deleteAll: p.accessS.DeleteAllLocks,
		}, withSkipPaginationTest())
	})

	t.Run("ListLocks", func(t *testing.T) {
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
			list: func(ctx context.Context, limit int, start string) ([]types.Lock, string, error) {
				return p.accessS.ListLocks(ctx, limit, start, nil)
			},
			cacheList: func(ctx context.Context, limit int, start string) ([]types.Lock, string, error) {
				return p.cache.ListLocks(ctx, limit, start, nil)
			},
			cacheGet:  p.cache.GetLock,
			update:    p.accessS.UpsertLock,
			deleteAll: p.accessS.DeleteAllLocks,
			Range: func(ctx context.Context, start, end string) iter.Seq2[types.Lock, error] {
				return p.accessS.RangeLocks(ctx, start, end, nil)
			},
			cacheRange: func(ctx context.Context, start, end string) iter.Seq2[types.Lock, error] {
				return p.cache.RangeLocks(ctx, start, end, nil)
			},
		})
	})

	t.Run("ListLocksWithFilter", func(t *testing.T) {
		filter := types.NewLockFilter(false, types.LockTarget{Role: "target-role"})

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
			list: func(ctx context.Context, limit int, start string) ([]types.Lock, string, error) {
				return p.accessS.ListLocks(ctx, limit, start, filter)
			},
			cacheList: func(ctx context.Context, limit int, start string) ([]types.Lock, string, error) {
				return p.cache.ListLocks(ctx, limit, start, filter)
			},
			cacheGet:  p.cache.GetLock,
			update:    p.accessS.UpsertLock,
			deleteAll: p.accessS.DeleteAllLocks,
			Range: func(ctx context.Context, start, end string) iter.Seq2[types.Lock, error] {
				return p.accessS.RangeLocks(ctx, start, end, filter)
			},
			cacheRange: func(ctx context.Context, start, end string) iter.Seq2[types.Lock, error] {
				return p.cache.RangeLocks(ctx, start, end, filter)
			},
		})
	})
}
