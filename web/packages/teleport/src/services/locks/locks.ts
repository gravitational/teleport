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

import { withGenericUnsupportedError } from '../version/unsupported';
import { ApiLock, CreateLockRequest, Lock, LockKind } from './types';

export const lockService = {
  async fetchLocks() {
    return api.get(cfg.getLockUrl({ action: 'list-v2' })).then(makeLocks);
  },

  async createLock(req: CreateLockRequest) {
    return api.put(cfg.getLockUrl({ action: 'create' }), req).then(makeLock);
  },

  async deleteLock(id: string) {
    return api.delete(cfg.getLockUrl({ action: 'delete', uuid: id }));
  },
};

export async function listLocks(
  variables: {
    inForceOnly?: boolean;
    targets?: { kind: string; name: string }[];
  },
  signal?: AbortSignal
): Promise<readonly Lock[]> {
  const path = cfg.getLockUrl({ action: 'list-v2' });

  const qs = new URLSearchParams();
  if (variables.targets) {
    for (const target of variables.targets) {
      qs.append('target', `${target.kind}|${target.name}`);
    }
  }

  if (variables.inForceOnly !== undefined) {
    qs.set('in_force_only', variables.inForceOnly ? 'true' : 'false');
  }

  try {
    const json = await api.get(`${path}?${qs.toString()}`, signal);
    return makeLocks(json);
  } catch (err) {
    // TODO(nicholasmarais1158) DELETE IN v20.0.0
    withGenericUnsupportedError(err, '19.0.0');
  }
}

export async function createLock(
  variables: CreateLockRequest,
  signal?: AbortSignal
) {
  const json = await api.put(
    cfg.getLockUrl({ action: 'create' }),
    variables,
    signal
  );
  return makeLock(json);
}

export async function deleteLock(
  variables: { uuid: string },
  signal?: AbortSignal
) {
  return api.deleteWithOptions(
    cfg.getLockUrl({ action: 'delete', uuid: variables.uuid }),
    {
      signal,
    }
  );
}

export function makeLocks(json: { items: ApiLock[] }): Lock[] {
  const { items = [] } = json ?? {};
  return items.map(makeLock);
}

function makeLock(json: ApiLock): Lock {
  const {
    name,
    message,
    expires,
    createdAt,
    createdBy,
    targets: targetLookup,
  } = json;

  let targets: Lock['targets'] = [];
  if (targetLookup) {
    targets = Object.entries<string>(targetLookup).map(([key, value]) => ({
      kind: key as LockKind,
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
