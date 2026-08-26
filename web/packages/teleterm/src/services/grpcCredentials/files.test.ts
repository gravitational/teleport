/**
 * @jest-environment node
 */
/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import timers from 'node:timers/promises';

import { readGrpcCert } from './files';

let tempDir: string;

beforeAll(async () => {
  tempDir = await fs.mkdtemp(path.join(os.tmpdir(), 'grpc-files-test'));
});

afterAll(async () => {
  await fs.rm(tempDir, { recursive: true, force: true });
});

beforeEach(() => {
  jest.restoreAllMocks();
});

describe('readGrpcCert', () => {
  it('reads the file if the file already exists', async () => {
    await fs.writeFile(path.join(tempDir, 'already-exists'), 'foobar');

    await expect(
      readGrpcCert(tempDir, 'already-exists').then(buffer => buffer.toString())
    ).resolves.toEqual('foobar');
  });

  it('reads the file when the file is created after starting a watcher', async () => {
    const readGrpcCertPromise = readGrpcCert(
      tempDir,
      'created-after-start'
    ).then(buffer => buffer.toString());
    await timers.setTimeout(10);

    await fs.writeFile(path.join(tempDir, 'created-after-start'), 'foobar');

    await expect(readGrpcCertPromise).resolves.toEqual('foobar');
  });

  it('returns an error if the file is not created within the timeout', async () => {
    await expect(
      readGrpcCert(tempDir, 'non-existent', { timeoutMs: 1 })
    ).rejects.toMatchObject({
      message: expect.stringContaining('within the timeout'),
    });
  });

  it('returns an error if stat fails', async () => {
    const expectedError = new Error('Something went wrong');
    jest.spyOn(fs, 'stat').mockRejectedValue(expectedError);

    await expect(
      readGrpcCert(
        tempDir,
        'non-existent',
        { timeoutMs: 100 } // Make sure that the test doesn't hang for 10s on failure.
      )
    ).rejects.toEqual(expectedError);
  });
});
