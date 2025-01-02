/**
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

import cfg from 'teleport/config';
import api from 'teleport/services/api';

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
