/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package local

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/lite"
)

func TestLockCRUD(t *testing.T) {
	ctx := context.Background()
	lite, err := lite.NewWithConfig(ctx, lite.Config{Path: t.TempDir()})
	require.NoError(t, err)

	access := NewAccessService(lite)

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
					cmpopts.IgnoreFields(types.Metadata{}, "ID")))
			}
		})
		t.Run("GetLocks with targets", func(t *testing.T) {
			t.Parallel()
			// Match both locks with the targets.
			locks, err := access.GetLocks(ctx, false, lock1.Target(), lock2.Target())
			require.NoError(t, err)
			require.Len(t, locks, 2)
			require.Empty(t, cmp.Diff([]types.Lock{lock1, lock2}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "ID")))

			// Match only one of the locks.
			roleTarget := types.LockTarget{Role: "role-A"}
			locks, err = access.GetLocks(ctx, false, lock1.Target(), roleTarget)
			require.NoError(t, err)
			require.Len(t, locks, 1)
			require.Empty(t, cmp.Diff([]types.Lock{lock1}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "ID")))

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
				cmpopts.IgnoreFields(types.Metadata{}, "ID")))

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
			cmpopts.IgnoreFields(types.Metadata{}, "ID")))
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
			cmpopts.IgnoreFields(types.Metadata{}, "ID")))
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
