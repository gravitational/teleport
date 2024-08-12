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

import type { FileStorage } from 'teleterm/services/fileStorage';

export function createMockFileStorage(opts?: {
  filePath: string;
}): FileStorage {
  let state = {};
  return {
    put(key: string, json: any) {
      state[key] = json;
    },

    get<T>(key?: string): T {
      return key ? state[key] : (state as T);
    },

    async write() {},

    replace(json: any) {
      state = json;
    },

    getFilePath() {
      return opts?.filePath || '';
    },

    getFileName() {
      return opts?.filePath.split('/').at(-1) || '';
    },

    getFileLoadingError() {
      return undefined;
    },
  };
}
