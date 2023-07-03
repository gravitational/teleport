/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import api from 'teleport/services/api';
import cfg from 'teleport/config';

import { CreateLockRequest, Lock } from './types';

export const lockService = {
  fetchLocks(): Promise<Lock[]> {
    return api.get(cfg.getLocksUrl()).then(makeLocks);
  },

  createLock(req: CreateLockRequest): Promise<Lock> {
    return api.put(cfg.getLocksUrl(), req).then(makeLock);
  },

  deleteLock(id: string): Promise<void> {
    return api.delete(cfg.getLocksUrlWithUuid(id));
  },
};

export function makeLocks(json: any): Lock[] {
  json = json || [];
  return json.map(makeLock);
}

function makeLock(json: any): Lock {
  json = json || {};
  const {
    name,
    message,
    expires,
    createdAt,
    createdBy,
    targets: targetLookup,
  } = json;

  let targets = [];
  if (targets) {
    targets = Object.entries(targetLookup).map(([key, value]) => ({
      kind: key,
      name: value,
    }));
  }

  return {
    name,
    message,
    expires,
    createdAt,
    createdBy,
    targets,
    targetLookup: {
      user: targetLookup?.user,
      role: targetLookup?.role,
      login: targetLookup?.login,
      node: targetLookup?.node,
      mfa_device: targetLookup?.mfa_device,
      windows_desktop: targetLookup?.windows_desktop,
      device: targetLookup?.device,
      access_request: targetLookup?.access_request,
    },
  };
}
