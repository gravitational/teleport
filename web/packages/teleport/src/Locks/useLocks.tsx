/*
Copyright 2023 Gravitational, Inc.

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

import { useCallback, useEffect, useState } from 'react';

import api from 'teleport/services/api';
import cfg from 'teleport/config';

import type { CreateLockData, Lock, LockForTable } from './types';

export function useLocks(clusterId: string) {
  const [locks, setLocks] = useState<LockForTable[]>([]);

  const fetchLocks = useCallback((clusterId: string) => {
    api.get(cfg.getLocksUrl(clusterId)).then((resp: Lock[]) => {
      const locksResp = resp.map(lock => ({
        ...lock,
        targets: Object.entries(lock.targets).map(([key, value]) => ({
          name: key,
          value,
        })),
      }));
      setLocks(locksResp);
    });
  }, []);

  const createLock = useCallback(
    async (clusterId: string, createLockData: CreateLockData) => {
      return await api.put(cfg.getLocksUrl(clusterId), createLockData);
    },
    []
  );

  useEffect(() => {
    fetchLocks(clusterId);
  }, [clusterId, fetchLocks]);

  return { createLock, fetchLocks, locks };
}
