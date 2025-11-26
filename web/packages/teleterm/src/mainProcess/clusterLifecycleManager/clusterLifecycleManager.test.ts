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

import { enablePatches } from 'immer';

import Logger, { NullService } from 'teleterm/logger';
import { IAwaitableSender } from 'teleterm/mainProcess/awaitableSender';
import { ClusterStore } from 'teleterm/mainProcess/clusterStore';
import { ProfileChangeSet } from 'teleterm/mainProcess/profileWatcher';
import { RendererIpc } from 'teleterm/mainProcess/types';
import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import { MockTshClient } from 'teleterm/services/tshd/fixtures/mocks';
import {
  makeLoggedInUser,
  makeRootCluster,
} from 'teleterm/services/tshd/testHelpers';

import {
  ClusterLifecycleEvent,
  ClusterLifecycleManager,
} from './clusterLifecycleManager';

beforeAll(() => {
  Logger.init(new NullService());
});

enablePatches();

const cluster = makeRootCluster();

const tests: {
  name: string;
  setup(opts: { tshdClient: MockTshClient }): Promise<{
    profileWatcher: () => AsyncGenerator<ProfileChangeSet, void, void>;
    throwInRendererHandler?: boolean;
  }>;
  expect(opts: {
    clusterStore: ClusterStore;
    tshdClient: MockTshClient;
    rendererHandler: IAwaitableSender<ClusterLifecycleEvent>;
    globalErrorHandler: jest.Mock;
  }): void;
}[] = [
  {
    name: 'when cluster is added, it updates state and notifies renderer',
    setup: async () => {
      return {
        profileWatcher: makeWatcher([{ op: 'added', cluster }]),
      };
    },
    expect: ({ clusterStore, rendererHandler }) => {
      expect(clusterStore.getState().get(cluster.uri)).toBeDefined();
      expect(rendererHandler.send).toHaveBeenCalledWith({
        op: 'did-add-cluster',
        uri: cluster.uri,
      });
    },
  },
  {
    name: 'when cluster is added and renderer fails, it updates state and reports an error',
    setup: async () => {
      return {
        throwInRendererHandler: true,
        profileWatcher: makeWatcher([{ op: 'added', cluster }]),
      };
    },
    expect: ({ clusterStore, globalErrorHandler }) => {
      expect(clusterStore.getState().get(cluster.uri)).toBeDefined();
      expect(globalErrorHandler).toHaveBeenCalledWith(
        RendererIpc.ProfileWatcherError,
        {
          error: new Error('Error in renderer'),
          reason: 'processing-error',
        }
      );
    },
  },
  {
    name: 'when cluster is removed, it updates state and notifies renderer',
    setup: async ({ tshdClient }) => {
      jest.spyOn(tshdClient, 'logout');
      jest
        .spyOn(tshdClient, 'listRootClusters')
        .mockResolvedValue(new MockedUnaryCall({ clusters: [cluster] }));
      return {
        profileWatcher: makeWatcher([{ op: 'removed', cluster }]),
      };
    },
    expect: ({ clusterStore, tshdClient, rendererHandler }) => {
      expect(clusterStore.getState().get(cluster.uri)).toBeUndefined();
      expect(tshdClient.logout).toHaveBeenCalledWith({
        clusterUri: cluster.uri,
        removeProfile: true,
      });
      expect(rendererHandler.send).toHaveBeenCalledWith({
        op: 'will-logout-and-remove',
        uri: cluster.uri,
      });
    },
  },
  {
    name: 'when cluster is removed and renderer fails, it keeps the cluster and reports an error',
    setup: async ({ tshdClient }) => {
      jest.spyOn(tshdClient, 'logout');
      jest
        .spyOn(tshdClient, 'listRootClusters')
        .mockResolvedValue(new MockedUnaryCall({ clusters: [cluster] }));
      return {
        throwInRendererHandler: true,
        profileWatcher: makeWatcher([{ op: 'removed', cluster }]),
      };
    },
    expect: ({ clusterStore, tshdClient, globalErrorHandler }) => {
      expect(clusterStore.getState().get(cluster.uri)).toBeDefined();
      expect(tshdClient.logout).not.toHaveBeenCalled();
      expect(globalErrorHandler).toHaveBeenCalledWith(
        RendererIpc.ProfileWatcherError,
        {
          error: new Error('Error in renderer'),
          reason: 'processing-error',
        }
      );
    },
  },
  {
    name: 'when cluster becomes logged-out, it updates state and notifies renderer',
    setup: async ({ tshdClient }) => {
      const next = makeRootCluster({
        connected: false,
        loggedInUser: makeLoggedInUser({ name: '' }),
      });
      jest.spyOn(tshdClient, 'logout');
      jest
        .spyOn(tshdClient, 'listRootClusters')
        .mockResolvedValue(new MockedUnaryCall({ clusters: [cluster] }));
      return {
        profileWatcher: makeWatcher([
          { op: 'changed', next, previous: cluster },
        ]),
      };
    },
    expect: ({ clusterStore, tshdClient, rendererHandler }) => {
      expect(clusterStore.getState().get(cluster.uri).loggedInUser.name).toBe(
        ''
      );
      expect(tshdClient.logout).toHaveBeenCalledWith({
        clusterUri: cluster.uri,
        removeProfile: false,
      });
      expect(rendererHandler.send).toHaveBeenCalledWith({
        op: 'will-logout',
        uri: cluster.uri,
      });
    },
  },
  {
    name: 'when cluster becomes logged-out and renderer fails, it keeps logged-in state and reports an error',
    setup: async ({ tshdClient }) => {
      const next = makeRootCluster({
        connected: false,
        loggedInUser: makeLoggedInUser({ name: '' }),
      });
      jest.spyOn(tshdClient, 'logout');
      jest
        .spyOn(tshdClient, 'listRootClusters')
        .mockResolvedValue(new MockedUnaryCall({ clusters: [cluster] }));
      return {
        throwInRendererHandler: true,
        profileWatcher: makeWatcher([
          { op: 'changed', next, previous: cluster },
        ]),
      };
    },
    expect: ({ clusterStore, tshdClient, globalErrorHandler }) => {
      expect(clusterStore.getState().get(cluster.uri).loggedInUser.name).toBe(
        cluster.loggedInUser.name
      );
      expect(tshdClient.logout).not.toHaveBeenCalled();
      expect(globalErrorHandler).toHaveBeenCalledWith(
        RendererIpc.ProfileWatcherError,
        {
          error: new Error('Error in renderer'),
          reason: 'processing-error',
        }
      );
    },
  },
  {
    name: 'when cluster changes, it updates state and clears stale clients',
    setup: async ({ tshdClient }) => {
      const next = makeRootCluster({
        connected: false,
      });
      jest
        .spyOn(tshdClient, 'listRootClusters')
        .mockResolvedValue(new MockedUnaryCall({ clusters: [cluster] }));
      jest.spyOn(tshdClient, 'clearStaleClusterClients');
      return {
        profileWatcher: makeWatcher([
          { op: 'changed', next, previous: cluster },
        ]),
      };
    },
    expect: ({ clusterStore, tshdClient, rendererHandler }) => {
      expect(clusterStore.getState().get(cluster.uri).connected).toBe(false);
      expect(tshdClient.clearStaleClusterClients).toHaveBeenCalledWith({
        rootClusterUri: cluster.uri,
      });
      expect(rendererHandler.send).not.toHaveBeenCalled();
    },
  },
  {
    name: 'when access of logged in user changes, it updates state, clears stale clients, and notifies renderer',
    setup: async ({ tshdClient }) => {
      const next = makeRootCluster({
        loggedInUser: makeLoggedInUser({ activeRequests: ['abcd'] }),
      });
      jest
        .spyOn(tshdClient, 'listRootClusters')
        .mockResolvedValue(new MockedUnaryCall({ clusters: [cluster] }));
      jest.spyOn(tshdClient, 'clearStaleClusterClients');
      return {
        profileWatcher: makeWatcher([
          { op: 'changed', next, previous: cluster },
        ]),
      };
    },
    expect: ({ clusterStore, tshdClient, rendererHandler }) => {
      expect(
        clusterStore.getState().get(cluster.uri).loggedInUser.activeRequests
      ).toEqual(['abcd']);
      expect(tshdClient.clearStaleClusterClients).toHaveBeenCalledWith({
        rootClusterUri: cluster.uri,
      });
      expect(rendererHandler.send).toHaveBeenCalledWith({
        op: 'did-change-access',
        uri: cluster.uri,
      });
    },
  },
  {
    name: 'when access of logged in user changes and renderer fails, it updates state, clears stale clients and reports error',
    setup: async ({ tshdClient }) => {
      const next = makeRootCluster({
        loggedInUser: makeLoggedInUser({ activeRequests: ['abcd'] }),
      });
      jest
        .spyOn(tshdClient, 'listRootClusters')
        .mockResolvedValue(new MockedUnaryCall({ clusters: [cluster] }));
      jest.spyOn(tshdClient, 'clearStaleClusterClients');
      return {
        throwInRendererHandler: true,
        profileWatcher: makeWatcher([
          { op: 'changed', next, previous: cluster },
        ]),
      };
    },
    expect: ({ clusterStore, tshdClient, globalErrorHandler }) => {
      expect(
        clusterStore.getState().get(cluster.uri).loggedInUser.activeRequests
      ).toEqual(['abcd']);
      expect(tshdClient.clearStaleClusterClients).toHaveBeenCalledWith({
        rootClusterUri: cluster.uri,
      });
      expect(globalErrorHandler).toHaveBeenCalledWith(
        RendererIpc.ProfileWatcherError,
        {
          error: new Error('Error in renderer'),
          reason: 'processing-error',
        }
      );
    },
  },
];

// eslint-disable-next-line jest/expect-expect
test.each(tests)('$name', async ({ setup, expect: testExpect }) => {
  const mockTshdClient = new MockTshClient();
  const mockAppUpdater = {
    maybeRemoveManagingCluster: jest.fn().mockResolvedValue(undefined),
  };
  const clusterStore = new ClusterStore(async () => mockTshdClient, {
    crashWindow: async () => {},
  });
  const globalErrorHandler = jest.fn();
  const windowsManager = {
    getWindow: () => ({
      webContents: {
        send: globalErrorHandler,
      },
    }),
  };
  const { profileWatcher, throwInRendererHandler } = await setup({
    tshdClient: mockTshdClient,
  });

  const done = Promise.withResolvers<void>();
  const consumer = (async function* () {
    try {
      for await (const value of profileWatcher()) {
        yield value;
      }
    } finally {
      done.resolve();
    }
  })();

  const manager = new ClusterLifecycleManager(
    clusterStore,
    async () => mockTshdClient,
    mockAppUpdater,
    windowsManager,
    consumer
  );
  const mockRendererHandler = {
    send: throwInRendererHandler
      ? jest.fn().mockRejectedValue(new Error('Error in renderer'))
      : jest.fn().mockResolvedValue(undefined),
    whenDisposed: () => new Promise<void>(() => {}),
  };
  manager.setRendererEventHandler(mockRendererHandler);
  await manager.syncRootClustersAndStartProfileWatcher();
  await done.promise;

  testExpect({
    globalErrorHandler,
    clusterStore,
    tshdClient: mockTshdClient,
    rendererHandler: mockRendererHandler,
  });
});

function makeWatcher(...events: ProfileChangeSet[]) {
  return async function* () {
    yield* events;
  };
}
