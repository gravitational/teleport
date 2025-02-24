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

// Both versions are imported because some operations need to be sync.
import fs from 'node:fs';
import fsAsync from 'node:fs/promises';
import path from 'node:path';

import { debounce } from 'shared/utils/highbar';

import Logger from 'teleterm/logger';

const logger = new Logger('FileStorage');

export interface FileStorage {
  /** Asynchronously updates value for a given key. */
  put(key: string, json: any): void;

  /** Asynchronously replaces the entire storage state with a new value. */
  replace(json: any): void;

  /** Asynchronously writes the storage state to disk. */
  write(): Promise<void>;

  /** Returns value for a given key. If the key is omitted, the entire storage state is returned. */
  // TODO(ravicious): Add a generic type to createFileStorage rather than returning `unknown`.
  // https://github.com/gravitational/teleport/pull/22728#discussion_r1129566566
  get(key?: string): unknown;

  /** Returns the file path used to create the storage. */
  getFilePath(): string;

  /** Returns the file name used to create the storage.
   *
   * Added so that ConfigService itself doesn't need to import node:path and can remain universal.
   */
  getFileName(): string;

  /** Returns the error that could occur while reading and parsing the file. */
  getFileLoadingError(): Error | undefined;
}

/**
 * createFileStorage reads and parses existing JSON structure from filePath or creates a new file
 * under filePath with an empty object if the file is missing.
 *
 * createFileStorage itself uses blocking filesystem APIs but the functions of the returned
 * FileStorage interface, such as write and replace, are async.
 */
// createFileStorage needs to be kept sync so that initialization of the app in main.ts can be sync.
// createFileStorage is called only during initialization, so blocking the main process during that
// time is acceptable.
//
// However, functions such as write or replace returned by createFileStorage need to be async as
// those are called after initialization.
export function createFileStorage(opts: {
  filePath: string;
  debounceWrites: boolean;
  /** Prevents state updates when the file has not been loaded correctly, so its content will not be overwritten. */
  discardUpdatesOnLoadError?: boolean;
}): FileStorage {
  if (!opts || !opts.filePath) {
    throw Error('missing filePath');
  }

  const { filePath } = opts;

  let state: any, error: Error | undefined;
  try {
    state = loadStateSync(filePath);
  } catch (e) {
    state = {};
    error = e;
    logger.error(`Cannot read ${filePath} file`, e);
  }

  const discardUpdates = error && opts.discardUpdatesOnLoadError;

  function put(key: string, json: any): void {
    if (discardUpdates) {
      return;
    }
    state[key] = json;
    stringifyAndWrite();
  }

  function write(): Promise<void> {
    if (discardUpdates) {
      return;
    }
    const text = stringify(state);
    return writeFile(filePath, text);
  }

  function replace(json: any): void {
    if (discardUpdates) {
      return;
    }
    state = json;
    stringifyAndWrite();
  }

  function get(key?: string): unknown {
    return key ? state[key] : state;
  }

  function getFilePath(): string {
    return opts.filePath;
  }

  function getFileName(): string {
    return path.basename(opts.filePath);
  }

  function getFileLoadingError(): Error | undefined {
    return error;
  }

  function stringifyAndWrite(): void {
    const text = stringify(state);

    opts.debounceWrites
      ? writeFileDebounced(filePath, text)
      : writeFile(filePath, text);
  }

  return {
    put,
    write,
    get,
    replace,
    getFilePath,
    getFileName,
    getFileLoadingError,
  };
}

function loadStateSync(filePath: string): any {
  const file = readOrCreateFileSync(filePath);
  return JSON.parse(file);
}

const defaultValue = '{}' as const;

function readOrCreateFileSync(filePath: string): string {
  try {
    return fs.readFileSync(filePath, { encoding: 'utf-8' });
  } catch (error) {
    if (error?.code === 'ENOENT') {
      fs.writeFileSync(filePath, defaultValue);
      return defaultValue;
    }
    throw error;
  }
}

function stringify(state: any) {
  return JSON.stringify(state, null, 2);
}

const writeFileDebounced = debounce(
  (filePath: string, text: string) => writeFile(filePath, text),
  2000
);

const writeFile = (filePath: string, text: string) =>
  fsAsync.writeFile(filePath, text).catch(error => {
    logger.error(`Cannot update ${filePath} file`, error);
  });
