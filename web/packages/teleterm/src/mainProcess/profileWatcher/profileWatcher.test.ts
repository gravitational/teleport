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

import fs from 'fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';

import { Cluster } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';
import { wait } from 'shared/utils/wait';

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { RootClusterUri, routing } from 'teleterm/ui/uri';

import { watchProfiles } from './profileWatcher';

let tshDir: string;

beforeAll(async () => {
  tshDir = await fs.mkdtemp(path.join(tmpdir(), 'profile-watcher-test'));
});

afterAll(async () => {
  if (tshDir) {
    await fs.rm(tshDir, { recursive: true, force: true });
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

async function mockTshClient(initial: { clusters: Cluster[] }) {
  const listRootClusters = async () => {
    const paths = await fs.readdir(tshDir);
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
  const tshClientMock = await mockTshClient({ clusters: [] });
  const clusterStoreMock = mockClusterStore({ clusters: [] });

  const watcher = watchProfiles(tshDir, tshClientMock, clusterStoreMock);

  const cluster = makeRootCluster();
  await tshClientMock.insertOrUpdateCluster(cluster);

  const { value } = await watcher.next();
  expect(value).toEqual([{ op: 'added', cluster }]);

  await watcher.return(undefined);
});

test('yields a "removed" change when cluster disappears', async () => {
  const cluster = makeRootCluster();
  const tshClientMock = await mockTshClient({ clusters: [cluster] });
  const clusterStoreMock = mockClusterStore({ clusters: [cluster] });

  const watcher = watchProfiles(tshDir, tshClientMock, clusterStoreMock);

  void tshClientMock.removeCluster(cluster.uri);

  const { value } = await watcher.next();
  expect(value).toEqual([{ op: 'removed', cluster }]);

  await watcher.return(undefined);
});

test('yields a "changed" change when cluster properties differ', async () => {
  const oldCluster = makeRootCluster();
  const tshClientMock = await mockTshClient({ clusters: [oldCluster] });
  const clusterStoreMock = mockClusterStore({ clusters: [oldCluster] });

  const watcher = watchProfiles(tshDir, tshClientMock, clusterStoreMock);

  const newCluster: Cluster = { ...oldCluster, connected: false };
  void tshClientMock.insertOrUpdateCluster(newCluster);

  const { value } = await watcher.next();
  expect(value).toEqual([
    { op: 'changed', previous: oldCluster, next: newCluster },
  ]);

  await watcher.return(undefined);
});

test('does not yield when no cluster changes detected', async () => {
  const cluster = makeRootCluster();
  const tshClientMock = await mockTshClient({
    clusters: [
      // Extend the cluster with properties loaded from the proxy to verify
      // if they are properly ignored when detecting changes.
      { ...cluster, authClusterId: 'some-id' },
    ],
  });
  const clusterStoreMock = mockClusterStore({ clusters: [cluster] });

  const watcher = watchProfiles(tshDir, tshClientMock, clusterStoreMock, {
    signal: abortController.signal,
  });

  // Overwrite the cluster (profile properties are unchanged).
  void tshClientMock.insertOrUpdateCluster(cluster);

  const race = Promise.race([
    watcher.next(),
    // Wait a little longer than the debounce value (200 ms).
    wait(250).then(() => 'timeout'),
  ]);

  expect(await race).toBe('timeout');
  // Cancel the watcher with the abort signal, it's blocked on `watcher.next()`.
  abortController.abort();
});

test('file system events are debounced and no events are lost when handler is slow', async () => {
  const debounceMs = 200; // Debounce interval used in watchProfiles
  const slowHandlerMs = 300; // Simulated slow handler duration
  const tshClientMock = await mockTshClient({ clusters: [] });
  const clusterStoreMock = mockClusterStore({ clusters: [] });

  const handler = jest.fn();
  const watcher = watchProfiles(tshDir, tshClientMock, clusterStoreMock, {
    signal: abortController.signal,
  });

  void (async () => {
    for await (let e of watcher) {
      await handler(e);
    }
  })();

  const cluster = makeRootCluster();

  handler.mockImplementation(() => Promise.resolve());

  // Insert two rapid events within debounce interval.
  await tshClientMock.insertOrUpdateCluster(cluster);
  await tshClientMock.insertOrUpdateCluster(cluster);

  // Wait slightly longer than debounce interval to ensure handler is called.
  await wait(debounceMs + 50);

  expect(handler).toHaveBeenCalledTimes(1);
  handler.mockClear();

  // Slow handler - ensure no events are lost.
  handler.mockImplementation(() => wait(slowHandlerMs));

  await tshClientMock.insertOrUpdateCluster(cluster);
  // Insert second event during slow handler processing (more than debounceMs but less than slowHandlerMs)
  await wait(debounceMs + 50);
  await tshClientMock.insertOrUpdateCluster(cluster);

  // Wait for both events to be processed.
  await wait(debounceMs + slowHandlerMs + 50);

  expect(handler).toHaveBeenCalledTimes(2);
});

test('watcher stops when consumer throws', async () => {
  const tshClientMock = await mockTshClient({ clusters: [] });
  const clusterStoreMock = mockClusterStore({ clusters: [] });

  const watcher = watchProfiles(tshDir, tshClientMock, clusterStoreMock);
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
  const cluster = makeRootCluster();
  const tshClientMock = await mockTshClient({ clusters: [] });
  const clusterStoreMock = mockClusterStore({ clusters: [cluster] });

  const watcher = watchProfiles(tshDir, tshClientMock, clusterStoreMock, {
    signal: abortController.signal,
  });
  const firstEvent = watcher.next();
  const secondEvent = watcher.next();

  await fs.rm(tshDir, { recursive: true });
  expect((await firstEvent).value).toEqual([{ op: 'removed', cluster }]);
  // Clean up the store, so that we can detect a change.
  clusterStoreMock.clearAll();

  await fs.mkdir(tshDir);
  await tshClientMock.insertOrUpdateCluster(cluster);

  expect((await secondEvent).value).toEqual([{ op: 'added', cluster }]);
});
