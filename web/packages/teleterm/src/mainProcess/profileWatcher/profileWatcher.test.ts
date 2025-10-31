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

import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';

import { Cluster } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';
import { wait } from 'shared/utils/wait';

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { RootClusterUri, routing } from 'teleterm/ui/uri';

import { watchProfiles } from './profileWatcher';

let tempDir: string;

beforeAll(async () => {
  tempDir = await fs.mkdtemp(path.join(os.tmpdir(), 'profile-watcher-test'));
});

afterAll(async () => {
  if (tempDir) {
    await fs.rm(tempDir, { recursive: true, force: true });
  }
});

// Ensure the watcher is stopped when a test fails.
let abortController: AbortController;
beforeEach(() => {
  abortController = new AbortController();
});
afterEach(() => {
  abortController.abort();
});

function makePerTestDir() {
  return fs.mkdtemp(path.join(tempDir, 'test'));
}

const testDebounceMs = 10;

async function mockTshClient(tshDir: string, initial: { clusters: Cluster[] }) {
  const listRootClusters = async () => {
    let paths: string[] = [];
    try {
      paths = await fs.readdir(tshDir);
    } catch (err) {
      if (err.code === 'ENOENT') {
        throw {
          name: 'TshdRpcError',
          code: 'NOT_FOUND',
        };
      }
      throw err;
    }
    return Promise.all(
      paths.map(async singlePath => {
        const file = await fs.readFile(
          path.join(tshDir, singlePath, 'cluster.json'),
          {
            encoding: 'utf-8',
          }
        );
        return Cluster.fromJsonString(file);
      })
    );
  };

  const insertOrUpdateCluster = async (cluster: Cluster) => {
    const profileDir = path.join(tshDir, routing.parseClusterName(cluster.uri));

    await fs.mkdir(profileDir, { recursive: true });
    await fs.writeFile(
      path.join(profileDir, 'cluster.json'),
      Cluster.toJsonString(cluster),
      { encoding: 'utf-8' }
    );
  };

  const removeCluster = async (uri: RootClusterUri) => {
    const profileDir = path.join(tshDir, routing.parseClusterName(uri));

    await fs.rm(profileDir, {
      recursive: true,
    });
  };

  // Set initial state.
  await Promise.all(
    initial.clusters.map(cluster => insertOrUpdateCluster(cluster))
  );

  return {
    listRootClusters,
    insertOrUpdateCluster,
    removeCluster,
  };
}

function mockClusterStore(initial: { clusters: Cluster[] }) {
  return {
    getRootClusters: () => initial.clusters,
    clearAll: () => {
      initial.clusters = [];
    },
  };
}

test('yields an "added" change when new cluster appears', async () => {
  const tshDir = await makePerTestDir();
  const tshClientMock = await mockTshClient(tshDir, { clusters: [] });
  const clusterStoreMock = mockClusterStore({ clusters: [] });
  const watcher = watchProfiles({
    tshDirectory: tshDir,
    tshClient: tshClientMock,
    clusterStore: clusterStoreMock,
    signal: abortController.signal,
    debounceMs: testDebounceMs,
  });

  const cluster = makeRootCluster();
  await tshClientMock.insertOrUpdateCluster(cluster);

  const { value } = await watcher.next();
  expect(value).toEqual([{ op: 'added', cluster }]);

  await watcher.return(undefined);
});

test('yields a "removed" change when cluster disappears', async () => {
  const tshDir = await makePerTestDir();
  const cluster = makeRootCluster();
  const tshClientMock = await mockTshClient(tshDir, { clusters: [cluster] });
  const clusterStoreMock = mockClusterStore({ clusters: [cluster] });
  const watcher = watchProfiles({
    tshDirectory: tshDir,
    tshClient: tshClientMock,
    clusterStore: clusterStoreMock,
    signal: abortController.signal,
    debounceMs: testDebounceMs,
  });

  void tshClientMock.removeCluster(cluster.uri);

  const { value } = await watcher.next();
  expect(value).toEqual([{ op: 'removed', cluster }]);

  await watcher.return(undefined);
});

test('yields a "changed" change when cluster properties differ', async () => {
  const tshDir = await makePerTestDir();
  const oldCluster = makeRootCluster();
  const tshClientMock = await mockTshClient(tshDir, { clusters: [oldCluster] });
  const clusterStoreMock = mockClusterStore({ clusters: [oldCluster] });
  const watcher = watchProfiles({
    tshDirectory: tshDir,
    tshClient: tshClientMock,
    clusterStore: clusterStoreMock,
    signal: abortController.signal,
    debounceMs: testDebounceMs,
  });

  const newCluster: Cluster = { ...oldCluster, connected: false };
  void tshClientMock.insertOrUpdateCluster(newCluster);

  const { value } = await watcher.next();
  expect(value).toEqual([
    { op: 'changed', previous: oldCluster, next: newCluster },
  ]);

  await watcher.return(undefined);
});

test('does not yield when no cluster changes detected', async () => {
  const tshDir = await makePerTestDir();
  const cluster = makeRootCluster();
  const tshClientMock = await mockTshClient(tshDir, {
    clusters: [
      // Extend the cluster with properties loaded from the proxy to verify
      // if they are properly ignored when detecting changes.
      { ...cluster, authClusterId: 'some-id' },
    ],
  });
  const clusterStoreMock = mockClusterStore({ clusters: [cluster] });
  const watcher = watchProfiles({
    tshDirectory: tshDir,
    tshClient: tshClientMock,
    clusterStore: clusterStoreMock,
    signal: abortController.signal,
    debounceMs: testDebounceMs,
  });

  // Overwrite the cluster (profile properties are unchanged).
  void tshClientMock.insertOrUpdateCluster(cluster);

  const race = Promise.race([
    watcher.next(),
    // Wait a little longer than the debounce value.
    wait(testDebounceMs + testDebounceMs / 2).then(() => 'timeout'),
  ]);

  expect(await race).toBe('timeout');
  // Cancel the watcher with the abort signal, it's blocked on `watcher.next()`.
  abortController.abort();
});

test('file system events are debounced and no events are lost when handler is slow', async () => {
  const tshDir = await makePerTestDir();
  const tshClientMock = await mockTshClient(tshDir, { clusters: [] });
  const clusterStoreMock = mockClusterStore({ clusters: [] });
  const handler = jest.fn();
  const testDebounceMs = 50;
  const watcher = watchProfiles({
    tshDirectory: tshDir,
    tshClient: tshClientMock,
    clusterStore: clusterStoreMock,
    signal: abortController.signal,
    debounceMs: testDebounceMs,
  });

  void (async () => {
    for await (let e of watcher) {
      await handler(e);
    }
  })();

  const cluster = makeRootCluster();

  // Test FS events debouncing.
  handler.mockImplementation(() => Promise.resolve());
  // Insert two rapid events within debounce interval.
  await tshClientMock.insertOrUpdateCluster(cluster);
  await tshClientMock.insertOrUpdateCluster(cluster);
  // Wait slightly longer than debounce interval to ensure a single handler is called.
  await wait(testDebounceMs + testDebounceMs / 2);
  expect(handler).toHaveBeenCalledTimes(1);
  handler.mockClear();

  // Test no events are lost when processing is slow.
  handler.mockImplementation(() => wait(2 * testDebounceMs));
  await tshClientMock.insertOrUpdateCluster(cluster);
  // Insert the second event while the first event is still processed
  // (it will finish at 2*testDebounceMs).
  await wait(testDebounceMs + testDebounceMs / 2);
  await tshClientMock.insertOrUpdateCluster(cluster);
  await expect(() => {
    return handler.mock.calls.length === 2;
  }).toEventuallyBeTrue({ waitFor: 1000, tick: 10 });
});

test('watcher stops when consumer throws', async () => {
  const tshDir = await makePerTestDir();
  const tshClientMock = await mockTshClient(tshDir, { clusters: [] });
  const clusterStoreMock = mockClusterStore({ clusters: [] });
  const watcher = watchProfiles({
    tshDirectory: tshDir,
    tshClient: tshClientMock,
    clusterStore: clusterStoreMock,
    signal: abortController.signal,
    debounceMs: testDebounceMs,
  });

  await expect(() =>
    watcher.throw(new Error('Consumer failure'))
  ).rejects.toThrow('Consumer failure');

  await tshClientMock.insertOrUpdateCluster(makeRootCluster());

  const race = await Promise.race([
    watcher.next(),
    wait(300).then(() => 'timeout'),
  ]);

  expect(race).toStrictEqual({ done: true, value: undefined });
});

test('removing tsh directory does not break watcher', async () => {
  const tshDir = await makePerTestDir();
  const cluster = makeRootCluster();
  const tshClientMock = await mockTshClient(tshDir, { clusters: [] });
  const clusterStoreMock = mockClusterStore({ clusters: [cluster] });
  const watcher = watchProfiles({
    tshDirectory: tshDir,
    tshClient: tshClientMock,
    clusterStore: clusterStoreMock,
    signal: abortController.signal,
    debounceMs: testDebounceMs,
  });

  // Start the watcher by pulling the first value.
  const firstEvent = watcher.next();
  await fs.rm(tshDir, { recursive: true });
  expect((await firstEvent).value).toEqual([{ op: 'removed', cluster }]);
  // Clean up the store, so that we can detect a change.
  clusterStoreMock.clearAll();

  await fs.mkdir(tshDir);
  await tshClientMock.insertOrUpdateCluster(cluster);
  // The second event needs to wait for the dir to be detected.
  jest.useFakeTimers();
  const secondEvent = watcher.next();
  // Polling uses 1 second interval.
  jest.advanceTimersByTime(1000);
  jest.useRealTimers();
  expect((await secondEvent).value).toEqual([{ op: 'added', cluster }]);
});

test('max file system events count is restricted', async () => {
  const tshDir = await makePerTestDir();
  const cluster = makeRootCluster();
  const tshClientMock = await mockTshClient(tshDir, { clusters: [] });
  const clusterStoreMock = mockClusterStore({ clusters: [] });
  const watcher = watchProfiles({
    tshDirectory: tshDir,
    tshClient: tshClientMock,
    clusterStore: clusterStoreMock,
    signal: abortController.signal,
    debounceMs: 50,
    maxFileSystemEvents: 2,
  });

  // Start the watcher by pulling the first value.
  const firstEvent = watcher.next();
  await tshClientMock.insertOrUpdateCluster(cluster);

  await expect(async () => await firstEvent).rejects.toThrow(
    `Exceeded file system event limit: more than 2 events detected within 50 ms`
  );
});
