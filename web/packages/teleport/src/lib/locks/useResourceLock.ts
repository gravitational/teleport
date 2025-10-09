/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useCallback } from 'react';

import { LockResourceKind } from 'teleport/LocksV2/NewLock/common';
import {
  createLock,
  deleteLock,
  listLocks,
} from 'teleport/services/locks/locks';
import { createQueryHook } from 'teleport/services/queryHelpers';
import useTeleport from 'teleport/useTeleport';

/**
 * `useResourceLock` provides the lock state for an individual resource. Lock
 * and unlock operations are provided, and permission are handled internally.
 *
 * Consider using this hook in combination with `ResourceLockDialog`,
 * `ResourceUnlockDialog` and `ResourceLockIndicator`.
 *
 * @param opts
 * @returns lock state and operations
 */
export function useResourceLock(opts: {
  /** The kind of resource to lock/unlock */
  targetKind: LockResourceKind;
  /** The name of the resource to lock/unlock */
  targetName: string;
  /** The stale time to pass to tanstack query */
  staleTime?: number;
}) {
  const { targetKind, targetName, staleTime = 30_000 } = opts;

  const ctx = useTeleport();
  const flags = ctx.getFeatureFlags();
  const hasListPermission = flags.locks;
  const hasAddPermission = flags.addLocks;
  const hasRemovePermission = flags.removeLocks;
  const queryClient = useQueryClient();

  const queryVars = {
    inForceOnly: true,
    targets: [{ kind: targetKind, name: targetName }],
  };

  const { isSuccess, data, error, isLoading } = useListLocks(queryVars, {
    enabled: hasListPermission,
    staleTime,
  });

  const {
    mutateAsync: removeLock,
    isPending: unlockPending,
    error: unlockError,
  } = useMutation({
    mutationFn: deleteLock,
    onSuccess: (_, vars) => {
      queryClient.setQueryData(listLocksQueryKey(queryVars), existingLocks => {
        return existingLocks?.filter(lock => lock.name !== vars.uuid);
      });
    },
  });

  const {
    mutateAsync: addLock,
    isPending: lockPending,
    error: lockError,
  } = useMutation({
    mutationFn: createLock,
    onSuccess: newLock => {
      queryClient.setQueryData(listLocksQueryKey(queryVars), existingLocks => {
        return existingLocks ? [...existingLocks, newLock] : [newLock];
      });
    },
  });

  // The lock/s can be removed if they all target the given resource, other
  // locks may affect other resources
  const canUnlock =
    hasRemovePermission &&
    data &&
    data.length > 0 &&
    data.reduce(
      (acc, lock) =>
        acc &&
        lock.targets.every(t => t.kind === targetKind && t.name === targetName),
      true
    );

  const unlock = useCallback(async () => {
    if (!canUnlock) throw new Error('missing permission to remove locks');
    return removeLock({ uuid: data[0].name });
  }, [canUnlock, data, removeLock]);

  const canLock = hasAddPermission;

  const lock = useCallback(
    async (message: string, ttl: string) => {
      if (!canLock) throw new Error('missing permission to create locks');
      return addLock({
        message,
        ttl,
        targets: { [targetKind]: targetName },
      });
    },
    [canLock, addLock, targetKind, targetName]
  );

  return {
    isLoading,
    isLocked: isSuccess && data.length > 0,
    locks: isSuccess ? data : null,
    error,
    canUnlock,
    /** Removes the lock. If there are multiple locks, only the first one is removed */
    unlock,
    unlockPending,
    unlockError,
    canLock,
    lock,
    lockPending,
    lockError,
  };
}

const { createQueryKey: listLocksQueryKey, useQuery: useListLocks } =
  createQueryHook(['locks', 'list'], listLocks);
