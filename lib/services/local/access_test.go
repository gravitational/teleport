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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestLockCRUD(t *testing.T) {
	ctx := context.Background()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	access := NewAccessService(backend)

	lock1, err := types.NewLock("lock1", types.LockSpecV2{
		Target: types.LockTarget{
			User: "user-A",
		},
	})
	require.NoError(t, err)

	lock2, err := types.NewLock("lock2", types.LockSpecV2{
		Target: types.LockTarget{
			Node: "node",
		},
	})
	require.NoError(t, err)

	t.Run("CreateLock", func(t *testing.T) {
		// Initially expect no locks to be returned.
		locks, err := access.GetLocks(ctx, false)
		require.NoError(t, err)
		require.Empty(t, locks)

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
