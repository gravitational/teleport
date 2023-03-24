/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import fs from 'fs/promises';

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

  /** Returns the error that could occur while reading and parsing the file. */
  getFileLoadingError(): Error | undefined;
}

export async function createFileStorage(opts: {
  filePath: string;
  debounceWrites: boolean;
  /** Prevents state updates when the file has not been loaded correctly, so its content will not be overwritten. */
  discardUpdatesOnLoadError?: boolean;
}): Promise<FileStorage> {
  if (!opts || !opts.filePath) {
    throw Error('missing filePath');
  }

  const { filePath } = opts;

  let state: any, error: Error | undefined;
  try {
    state = await loadState(filePath);
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
    writeFile(filePath, text);
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
    getFileLoadingError,
  };
}

async function loadState(filePath: string): Promise<any> {
  const file = await readOrCreateFile(filePath);
  return JSON.parse(file);
}

async function readOrCreateFile(filePath: string): Promise<string> {
  try {
    return await fs.readFile(filePath, { encoding: 'utf-8' });
  } catch (error) {
    const defaultValue = '{}';
    if (error?.code === 'ENOENT') {
      await fs.writeFile(filePath, defaultValue);
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
  fs.writeFile(filePath, text).catch(error => {
    logger.error(`Cannot update ${filePath} file`, error);
  });
