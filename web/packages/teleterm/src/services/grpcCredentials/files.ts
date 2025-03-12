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

import { watch, type Stats } from 'fs';
import { readFile, rename, stat, writeFile } from 'fs/promises';
import path from 'path';

import { wait } from 'shared/utils/wait';

import { makeCert } from './makeCert';

/**
 * Generates self-signed cert and saves it in the `certsDir`
 * using `certName` (only the cert is saved).
 * The generated pair (cert and key) is returned.
 */
export async function generateAndSaveGrpcCert(
  certsDir: string,
  certName: string
): Promise<{ cert: Buffer; key: Buffer }> {
  const createdCert = await makeCert({
    commonName: 'localhost',
    validityDays: 365,
  });

  // File is first saved using under `tempFullPath` and then renamed to `fullPath`.
  // It prevents from reading half written file.
  const fullPath = path.join(certsDir, certName);
  const tempFullPath = fullPath + '.tmp';
  await writeFile(tempFullPath, createdCert.cert);
  await rename(tempFullPath, fullPath);

  return {
    cert: Buffer.from(createdCert.cert),
    key: Buffer.from(createdCert.key),
  };
}

/**
 * Reads a cert with given `certName` in the `certDir`.
 * If the file doesn't exist, by default it will wait up to 10 seconds for it.
 */
export async function readGrpcCert(
  certsDir: string,
  certName: string,
  { timeoutMs = 10_000 } = {}
): Promise<Buffer> {
  const fullPath = path.join(certsDir, certName);
  const abortController = new AbortController();

  async function fileExistsAndHasSize(): Promise<boolean> {
    let stats: Stats;
    try {
      stats = await stat(fullPath);
    } catch (error) {
      if (error?.code === 'ENOENT') {
        return false;
      }
      throw error;
    }

    return !!stats.size;
  }

  function waitForFile(): Promise<Buffer> {
    return new Promise((resolve, reject) => {
      wait(timeoutMs, abortController.signal).then(
        () =>
          reject(
            new Error(
              `Could not read ${certName} certificate within the timeout.`
            )
          ),
        () => {} // Ignore abort errors.
      );

      // Watching must be started before checking if the file already exists to avoid race
      // conditions. If we checked if the file exists and then started the watcher, the file could
      // in theory be created between those two actions.
      watch(
        certsDir,
        { signal: abortController.signal },
        async (_, filename) => {
          if (certName === filename && (await fileExistsAndHasSize())) {
            resolve(readFile(fullPath));
          }
        }
      );

      fileExistsAndHasSize().then(
        exists => exists && resolve(readFile(fullPath)),
        err => reject(err)
      );
    });
  }

  try {
    return await waitForFile();
  } finally {
    abortController.abort();
  }
}
