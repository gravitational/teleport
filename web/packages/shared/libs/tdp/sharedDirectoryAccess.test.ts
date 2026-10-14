/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { selectDirectoryInBrowser } from './sharedDirectoryAccess';

type Deferred<T> = {
  promise: Promise<T>;
  resolve: (value: T) => void;
};

function defer<T>(): Deferred<T> {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>(res => {
    resolve = res;
  });
  return { promise, resolve };
}

/**
 * Flushes the microtask queue (and any already-queued macrotasks) so that
 * every operation that *can* make progress has done so before we assert.
 */
function flush(): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, 0));
}

/**
 * Builds a BrowserFileSystem backed by a single file whose `getFile()` call
 * parks on a per-invocation gate. Each `read()` must go through `getFile()`,
 * so the gates let a test observe exactly how far each concurrent `read()`
 * has progressed, and release them one at a time.
 */
async function makeGatedFileSystem() {
  // Records the order in which getFile() invocations begin.
  const getFileStarts: number[] = [];
  // One gate per getFile() invocation; resolving it lets that read finish.
  const gates: Deferred<void>[] = [];

  const fileLike = {
    size: 0,
    lastModified: 0,
    slice: () => ({
      arrayBuffer: async () => new ArrayBuffer(0),
    }),
  };

  const fileHandle = {
    name: 'file',
    kind: 'file',
    getFile: async () => {
      const gate = defer<void>();
      getFileStarts.push(gates.length);
      gates.push(gate);
      await gate.promise;
      return fileLike;
    },
  };

  const dirHandle = {
    name: 'root',
    kind: 'directory',
    // Fresh async iterator per walkPath() call.
    async *values() {
      yield fileHandle;
    },
  };

  window.showDirectoryPicker = jest
    .fn()
    .mockResolvedValue(dirHandle as unknown as FileSystemDirectoryHandle);

  const fs = await selectDirectoryInBrowser();
  return { fs, getFileStarts, gates };
}

test('serialized() runs operations one at a time, in order', async () => {
  const { fs, getFileStarts, gates } = await makeGatedFileSystem();
  const serial = fs.serialized();

  // Kick off two reads back-to-back on the serialized proxy.
  const first = serial.read('file', 0n, 1);
  const second = serial.read('file', 0n, 1);

  // Let everything that can run, run. Because the proxy serializes, only the
  // first read may reach getFile(); the second is stuck behind the queue.
  await flush();
  expect(getFileStarts).toEqual([0]);

  // Releasing the first read lets it complete, which unblocks the second.
  gates[0].resolve();
  await first;
  await flush();
  expect(getFileStarts).toEqual([0, 1]);

  gates[1].resolve();
  await expect(second).resolves.toEqual(new Uint8Array(0));
});

test('unserialized operations run concurrently', async () => {
  const { fs, getFileStarts, gates } = await makeGatedFileSystem();

  // Same two reads, but straight against the instance (not serialized).
  const first = fs.read('file', 0n, 1);
  const second = fs.read('file', 0n, 1);

  // Both reads reach getFile() before either is released, proving the gating
  // harness itself permits concurrency — so the serialization observed in the
  // test above is imposed by serialized(), not by the mock.
  await flush();
  expect(getFileStarts).toEqual([0, 1]);

  gates[0].resolve();
  gates[1].resolve();
  await Promise.all([first, second]);
});
