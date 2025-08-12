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

import { http, HttpResponse } from 'msw';

import cfg from 'teleport/config';
import { ApiLock } from 'teleport/services/locks/types';

export const listV2LocksSuccess = (options?: { locks?: ApiLock[] }) => {
  const { locks = [] } = options ?? {};
  return http.get(cfg.api.locks.listV2, () => {
    return HttpResponse.json(
      {
        items: locks,
      },
      { status: 200 }
    );
  });
};

export const listV2LocksError = (status: number, error: string | null = null) =>
  http.get(cfg.api.locks.listV2, () => {
    return HttpResponse.json({ error: { message: error } }, { status });
  });

export const removeLockSuccess = () => {
  return http.delete(cfg.api.locks.delete, () => {
    return HttpResponse.json('', { status: 200 });
  });
};

export const createLockSuccess = (lock: ApiLock) => {
  return http.put(cfg.api.locks.create, () => {
    return HttpResponse.json(lock, { status: 200 });
  });
};
