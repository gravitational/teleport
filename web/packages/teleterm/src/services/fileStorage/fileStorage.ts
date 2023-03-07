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

import fs, { existsSync, readFileSync, writeFileSync } from 'fs';

import { debounce } from 'shared/utils/highbar';

import Logger from 'teleterm/logger';

const logger = new Logger('FileStorage');

export interface FileStorage {
  /** Asynchronously updates value for a given key. */
  put(key: string, json: any): void;

  /** Asynchronously replaces the entire storage state with a new value. */
  replace(json: any): void;

  /** Synchronously writes the storage state to disk. */
  writeSync(): void;

  /** Returns value for a given key. If the key is omitted, the entire storage state is returned. */
  get<T>(key?: string): T;

  /** Returns the file path used to create the storage. */
  getFilePath(): string;
}

export function createFileStorage(opts: {
  filePath: string;
  debounceWrites: boolean;
}): FileStorage {
  if (!opts || !opts.filePath) {
    throw Error('missing filePath');
  }

  const { filePath } = opts;
  let state = loadState(opts.filePath);

  function put(key: string, json: any): void {
    state[key] = json;
    stringifyAndWrite();
  }

  function writeSync(): void {
    const text = stringify(state);
    try {
      fs.writeFileSync(filePath, text);
    } catch (error) {
      logger.error(`Cannot update ${filePath} file`, error);
    }
  }

  function get<T>(key?: string): T {
    return key ? state[key] : (state as T);
  }

  function replace(json: any): void {
    state = json;
    stringifyAndWrite();
  }

  function getFilePath(): string {
    return opts.filePath;
  }

  function stringifyAndWrite(): void {
    const text = stringify(state);

    opts.debounceWrites
      ? writeFileDebounced(filePath, text)
      : writeFile(filePath, text);
  }

  return {
    put,
    writeSync,
    get,
    replace,
    getFilePath,
  };
}

function loadState(filePath: string) {
  try {
    if (!existsSync(filePath)) {
      writeFileSync(filePath, '{}');
    }

    return JSON.parse(readFileSync(filePath, { encoding: 'utf-8' }));
  } catch (error) {
    logger.error(`Cannot read ${filePath} file`, error);
    return {};
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
  fs.promises.writeFile(filePath, text).catch(error => {
    logger.error(`Cannot update ${filePath} file`, error);
  });
