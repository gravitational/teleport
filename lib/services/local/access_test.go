/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package local

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

func TestRoleNotFound(t *testing.T) {
	t.Parallel()

	backend, err := memory.New(memory.Config{
		Context: t.Context(),
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	access := NewAccessService(backend)

	_, err = access.GetRole(t.Context(), "test-role")
	assert.Error(t, err)
	assert.True(t, trace.IsNotFound(err))
	assert.Equal(t, "role test-role is not found", err.Error())
}

func TestLockCRUD(t *testing.T) {
	t.Parallel()

	newLockFilter := func(inForceOnly bool, targets ...types.LockTarget) *types.LockFilter {
		filter := &types.LockFilter{
			InForceOnly: inForceOnly,
			Targets:     make([]*types.LockTarget, 0, len(targets)),
		}
		for _, tgt := range targets {
			filter.Targets = append(filter.Targets, &tgt)
		}
		return filter
	}

	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	access := NewAccessService(backend)
	expireTime1 := clock.Now().Add(1 * time.Hour)
	expireTime2 := clock.Now().Add(5 * time.Hour)

	lock1, err := types.NewLock("lock1", types.LockSpecV2{
		Target: types.LockTarget{
			User: "user-A",
		},
		Expires: &expireTime1,
	})
	require.NoError(t, err)

	lock2, err := types.NewLock("lock2", types.LockSpecV2{
		Target: types.LockTarget{
			ServerID: "node",
		},
		Expires: &expireTime2,
	})
	require.NoError(t, err)

	t.Run("CreateLock", func(t *testing.T) {
		// Initially expect no locks to be returned.
		locks, err := access.GetLocks(ctx, false)
		require.NoError(t, err)
		require.Empty(t, locks)

		locks, err = stream.Collect(access.RangeLocks(ctx, "", "", nil /* no filter */))
		require.NoError(t, err)
		require.Empty(t, locks)

		locks, next, err := access.ListLocks(ctx, 0, "", nil /* no filter */)
		require.NoError(t, err)
		require.Empty(t, locks)
		require.Empty(t, next)

		// Create locks.
		err = access.UpsertLock(ctx, lock1)
		require.NoError(t, err)
		err = access.UpsertLock(ctx, lock2)
		require.NoError(t, err)
	})

	// Run LockGetters in nested subtests to allow parallelization.
	t.Run("LockGetters", func(t *testing.T) {
		t.Run("GetLocks", func(t *testing.T) {
			t.Parallel()
			for _, inForceOnly := range []bool{true, false} {
				locks, err := access.GetLocks(ctx, inForceOnly)
				require.NoError(t, err)
				require.Len(t, locks, 2)
				require.Empty(t, cmp.Diff([]types.Lock{lock1, lock2}, locks,
					cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
			}
		})
		t.Run("ListLocks", func(t *testing.T) {
			t.Parallel()
			for _, inForceOnly := range []bool{true, false} {
				locks, next, err := access.ListLocks(ctx, 0, "", newLockFilter(inForceOnly))
				require.Empty(t, next)
				require.NoError(t, err)
				require.Len(t, locks, 2)
				require.Empty(t, cmp.Diff([]types.Lock{lock1, lock2}, locks,
					cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

				page1, page2Start, err := access.ListLocks(ctx, 1, "", newLockFilter(inForceOnly))
				require.NotEmpty(t, page2Start)
				require.NoError(t, err)
				require.Len(t, page1, 1)
				require.Empty(t, cmp.Diff([]types.Lock{lock1}, page1,
					cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

				page2, next, err := access.ListLocks(ctx, 0, page2Start, newLockFilter(inForceOnly))
				require.Empty(t, next)
				require.NoError(t, err)
				require.Empty(t, cmp.Diff([]types.Lock{lock2}, page2,
					cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

				require.Empty(t, cmp.Diff([]types.Lock{lock1, lock2}, append(page1, page2...),
					cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

			}
		})
		t.Run("RangeLocks", func(t *testing.T) {
			t.Parallel()
			for _, inForceOnly := range []bool{true, false} {
				locks, err := stream.Collect(access.RangeLocks(ctx, "", "", newLockFilter(inForceOnly)))
				require.NoError(t, err)
				require.Len(t, locks, 2)
				require.Empty(t, cmp.Diff([]types.Lock{lock1, lock2}, locks,
					cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

				_, page2Start, _ := access.ListLocks(ctx, 1, "", newLockFilter(inForceOnly))

				page1, err := stream.Collect(access.RangeLocks(ctx, "", page2Start, newLockFilter(inForceOnly)))
				require.NoError(t, err)
				require.Len(t, page1, 1)
				require.Empty(t, cmp.Diff([]types.Lock{lock1}, page1,
					cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

				page2, err := stream.Collect(access.RangeLocks(ctx, page2Start, "", newLockFilter(inForceOnly)))
				require.NoError(t, err)
				require.Len(t, page2, 1)
				require.Empty(t, cmp.Diff([]types.Lock{lock2}, page2,
					cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
			}
		})
		t.Run("GetLocks with targets", func(t *testing.T) {
			t.Parallel()
			// Match both locks with the targets.
			locks, err := access.GetLocks(ctx, false, lock1.Target(), lock2.Target())
			require.NoError(t, err)
			require.Len(t, locks, 2)
			require.Empty(t, cmp.Diff([]types.Lock{lock1, lock2}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

			// Match only one of the locks.
			roleTarget := types.LockTarget{Role: "role-A"}
			locks, err = access.GetLocks(ctx, false, lock1.Target(), roleTarget)
			require.NoError(t, err)
			require.Len(t, locks, 1)
			require.Empty(t, cmp.Diff([]types.Lock{lock1}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

			// Match none of the locks.
			locks, err = access.GetLocks(ctx, false, roleTarget)
			require.NoError(t, err)
			require.Empty(t, locks)
		})
		t.Run("ListLocks with targets", func(t *testing.T) {
			t.Parallel()
			// Match both locks with the targets.
			locks, next, err := access.ListLocks(ctx, 0, "", newLockFilter(false, lock1.Target(), lock2.Target()))
			require.NoError(t, err)
			require.Empty(t, next)
			require.Len(t, locks, 2)
			require.Empty(t, cmp.Diff([]types.Lock{lock1, lock2}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

			// Match only one of the locks.
			roleTarget := types.LockTarget{Role: "role-A"}
			locks, next, err = access.ListLocks(ctx, 0, "", newLockFilter(false, lock1.Target(), roleTarget))
			require.NoError(t, err)
			require.Empty(t, next)
			require.Len(t, locks, 1)
			require.Empty(t, cmp.Diff([]types.Lock{lock1}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

			// Match none of the locks.
			locks, next, err = access.ListLocks(ctx, 0, "", newLockFilter(false, roleTarget))
			require.NoError(t, err)
			require.Empty(t, next)
			require.Empty(t, locks)
		})
		t.Run("RangeLocks with targets", func(t *testing.T) {
			t.Parallel()
			// Match both locks with the targets.
			locks, err := stream.Collect(access.RangeLocks(ctx, "", "", newLockFilter(false, lock1.Target(), lock2.Target())))
			require.NoError(t, err)
			require.Len(t, locks, 2)
			require.Empty(t, cmp.Diff([]types.Lock{lock1, lock2}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

			// Match only one of the locks.
			roleTarget := types.LockTarget{Role: "role-A"}
			locks, err = stream.Collect(access.RangeLocks(ctx, "", "", newLockFilter(false, lock1.Target(), roleTarget)))
			require.NoError(t, err)
			require.Len(t, locks, 1)
			require.Empty(t, cmp.Diff([]types.Lock{lock1}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

			// Match none of the locks.
			locks, err = stream.Collect(access.RangeLocks(ctx, "", "", newLockFilter(false, roleTarget)))
			require.NoError(t, err)
			require.Empty(t, locks)
		})
		t.Run("GetLock", func(t *testing.T) {
			t.Parallel()
			// Get one of the locks.
			lock, err := access.GetLock(ctx, lock1.GetName())
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(lock1, lock,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

			// Attempt to get a nonexistent lock.
			_, err = access.GetLock(ctx, "lock3")
			require.Error(t, err)
			require.True(t, trace.IsNotFound(err))
		})
	})

	// Advance time and check some locks are no longer in force
	clock.Advance(2 * time.Hour)

	t.Run("LockGettersInForce", func(t *testing.T) {
		t.Run("GetLocks", func(t *testing.T) {
			t.Parallel()
			locks, err := access.GetLocks(ctx, true)
			require.NoError(t, err)
			require.Len(t, locks, 1)
			require.Empty(t, cmp.Diff([]types.Lock{lock2}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
		})
		t.Run("ListLocks", func(t *testing.T) {
			t.Parallel()
			// Only return in force locks
			locks, next, err := access.ListLocks(ctx, 0, "", newLockFilter(true))
			require.Empty(t, next)
			require.NoError(t, err)
			require.Len(t, locks, 1)
			require.Empty(t, cmp.Diff([]types.Lock{lock2}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

			// No filter returns all
			locks, next, err = access.ListLocks(ctx, 0, "", nil)
			require.Empty(t, next)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff([]types.Lock{lock1, lock2}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

		})
		t.Run("RangeLocks", func(t *testing.T) {
			t.Parallel()
			// Only return in force locks
			locks, err := stream.Collect(access.RangeLocks(ctx, "", "", newLockFilter(true)))
			require.NoError(t, err)
			require.Len(t, locks, 1)
			require.Empty(t, cmp.Diff([]types.Lock{lock2}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

			// No filter returns all
			locks, err = stream.Collect(access.RangeLocks(ctx, "", "", nil))
			require.NoError(t, err)
			require.Empty(t, cmp.Diff([]types.Lock{lock1, lock2}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

		})

		t.Run("GetLocks with targets", func(t *testing.T) {
			t.Parallel()
			locks, err := access.GetLocks(ctx, true, lock1.Target(), lock2.Target())
			require.NoError(t, err)
			require.Len(t, locks, 1)
			require.Empty(t, cmp.Diff([]types.Lock{lock2}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

			roleTarget := types.LockTarget{Role: "role-A"}
			locks, err = access.GetLocks(ctx, true, lock1.Target(), roleTarget)
			require.NoError(t, err)
			require.Empty(t, locks)
			require.Empty(t, locks)

			// Match none of the locks.
			locks, err = access.GetLocks(ctx, true, roleTarget)
			require.NoError(t, err)
			require.Empty(t, locks)
		})
		t.Run("ListLocks with targets", func(t *testing.T) {
			t.Parallel()

			// Match all but only lock2 is in force
			locks, next, err := access.ListLocks(ctx, 0, "", newLockFilter(true, lock1.Target(), lock2.Target()))
			require.NoError(t, err)
			require.Empty(t, next)
			require.Len(t, locks, 1)
			require.Empty(t, cmp.Diff([]types.Lock{lock2}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

			// Target match but not in force
			roleTarget := types.LockTarget{Role: "role-A"}
			locks, next, err = access.ListLocks(ctx, 0, "", newLockFilter(true, lock1.Target(), roleTarget))
			require.NoError(t, err)
			require.Empty(t, next)
			require.Empty(t, locks)

			// Nothing matched when some locks are in force
			locks, next, err = access.ListLocks(ctx, 0, "", newLockFilter(true, roleTarget))
			require.NoError(t, err)
			require.Empty(t, next)
			require.Empty(t, locks)
		})
		t.Run("RangeLocks with targets", func(t *testing.T) {
			t.Parallel()
			// Match all but only lock2 is in force
			locks, err := stream.Collect(access.RangeLocks(ctx, "", "", newLockFilter(true, lock1.Target(), lock2.Target())))
			require.NoError(t, err)
			require.Len(t, locks, 1)
			require.Empty(t, cmp.Diff([]types.Lock{lock2}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

			// Target match but not in force
			roleTarget := types.LockTarget{Role: "role-A"}
			locks, err = stream.Collect(access.RangeLocks(ctx, "", "", newLockFilter(true, lock1.Target(), roleTarget)))
			require.NoError(t, err)
			require.Empty(t, locks)

			// Nothing matched when some locks are in force
			locks, err = stream.Collect(access.RangeLocks(ctx, "", "", newLockFilter(true, roleTarget)))
			require.NoError(t, err)
			require.Empty(t, locks)
		})
	})

	t.Run("UpsertLock", func(t *testing.T) {
		// Get one of the locks.
		lock, err := access.GetLock(ctx, lock1.GetName())
		require.NoError(t, err)
		require.Empty(t, lock.Message())

		msg := "cluster maintenance"
		lock1.SetMessage(msg)
		err = access.UpsertLock(ctx, lock1)
		require.NoError(t, err)

		lock, err = access.GetLock(ctx, lock1.GetName())
		require.NoError(t, err)
		require.Equal(t, msg, lock.Message())
	})

	t.Run("DeleteLock", func(t *testing.T) {
		// Delete lock.
		err = access.DeleteLock(ctx, lock1.GetName())
		require.NoError(t, err)

		// Expect lock not found.
		_, err := access.GetLock(ctx, lock1.GetName())
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})

	t.Run("DeleteAllLocks", func(t *testing.T) {
		// Delete all locks.
		err = access.DeleteAllLocks(ctx)
		require.NoError(t, err)

		// Expect no locks to be returned.
		locks, err := access.GetLocks(ctx, false)
		require.NoError(t, err)
		require.Empty(t, locks)
	})

	t.Run("ReplaceRemoteLocks", func(t *testing.T) {
		clusterName := "root-cluster"

		newRemoteLocks := []types.Lock{lock1, lock2}
		err = access.ReplaceRemoteLocks(ctx, clusterName, newRemoteLocks)
		require.NoError(t, err)
		locks, err := access.GetLocks(ctx, false)
		require.NoError(t, err)
		require.Len(t, locks, 2)
		require.Empty(t, cmp.Diff(newRemoteLocks, locks,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
		for _, lock := range locks {
			require.True(t, strings.HasPrefix(lock.GetName(), clusterName+"/"))
		}

		// DeleteLock should work with remote locks.
		require.NoError(t, access.DeleteLock(ctx, lock1.GetName()))

		newRemoteLocks = []types.Lock{lock1}
		err = access.ReplaceRemoteLocks(ctx, clusterName, newRemoteLocks)
		require.NoError(t, err)
		locks, err = access.GetLocks(ctx, false)
		require.NoError(t, err)
		require.Len(t, locks, 1)
		require.Empty(t, cmp.Diff(newRemoteLocks, locks,
			cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
		_, err = access.GetLock(ctx, lock2.GetName())
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))

		err = access.ReplaceRemoteLocks(ctx, clusterName, nil)
		require.NoError(t, err)
		locks, err = access.GetLocks(ctx, false)
		require.NoError(t, err)
		require.Empty(t, locks)
	})
}

func Test_matchLock(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	expiredTime := clock.Now().Add(-2 * time.Hour)
	futureTime := clock.Now().Add(time.Hour)
	nowTime := clock.Now()

	tests := []struct {
		name   string
		lock   types.Lock
		filter *types.LockFilter
		want   bool
	}{
		{
			name: "nil filter matches",
			want: true,
		},
		{
			name: "inactive lock is not matched when InForceOnly is set",
			lock: &types.LockV2{
				Spec: types.LockSpecV2{
					Expires: &expiredTime,
				},
			},
			filter: &types.LockFilter{
				InForceOnly: true,
			},
			want: false,
		},
		{
			name: "active lock is matched when InForceOnly is set",
			lock: &types.LockV2{
				Spec: types.LockSpecV2{
					Expires: &futureTime,
				},
			},
			filter: &types.LockFilter{
				InForceOnly: true,
			},
			want: true,
		},
		{
			name: "no targets is a match when InForceOnly is false regardless of lock",
			filter: &types.LockFilter{
				InForceOnly: false,
			},
			want: true,
		},
		{
			name: "targets given do not match",
			filter: &types.LockFilter{
				InForceOnly: false,
				Targets: []*types.LockTarget{
					{
						User: "alice",
					},
					{
						User: "bob",
					},
				},
			},
			lock: &types.LockV2{
				Spec: types.LockSpecV2{
					Expires: &futureTime,
					Target: types.LockTarget{
						User: "charlie",
					},
				},
			},

			want: false,
		},
		{
			name: "targets given match",
			filter: &types.LockFilter{
				InForceOnly: false,
				Targets: []*types.LockTarget{
					{
						User: "alice",
					},
					{
						User: "bob",
					},
					{
						User: "charlie",
					},
				},
			},
			lock: &types.LockV2{
				Spec: types.LockSpecV2{
					Expires: &futureTime,
					Target: types.LockTarget{
						User: "charlie",
					},
				},
			},

			want: true,
		},
		{
			name: "targets given match but lock is expired with InForceOnly set",
			filter: &types.LockFilter{
				InForceOnly: true,
				Targets: []*types.LockTarget{
					{
						User: "alice",
					},
					{
						User: "bob",
					},
					{
						User: "charlie",
					},
				},
			},
			lock: &types.LockV2{
				Spec: types.LockSpecV2{
					Expires: &expiredTime,
					Target: types.LockTarget{
						User: "charlie",
					},
				},
			},

			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := matchLock(tt.lock, tt.filter, nowTime)
			require.Equal(t, tt.want, got)
		})
	}
}
