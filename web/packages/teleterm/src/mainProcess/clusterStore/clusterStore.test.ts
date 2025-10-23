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

import { enableMapSet, enablePatches } from 'immer';

import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import { MockTshClient } from 'teleterm/services/tshd/fixtures/mocks';
import {
  makeLeafCluster,
  makeRootCluster,
} from 'teleterm/services/tshd/testHelpers';

import { ClusterStore } from './clusterStore';

enablePatches();
enableMapSet();

const cluster = makeRootCluster({ connected: false });
const clusterWithDetails = makeRootCluster({
  ...cluster,
  connected: true,
  features: {
    advancedAccessWorkflows: true,
    isUsageBasedBilling: true,
  },
});
const leafCluster = makeLeafCluster();
const mockWindowsManager = {
  crashWindow: async () => undefined,
};

test('adds cluster', async () => {
  const mockClient = new MockTshClient();
  mockClient.addCluster = () => new MockedUnaryCall(cluster);
  const clusterStore = new ClusterStore(
    () => Promise.resolve(mockClient),
    mockWindowsManager
  );

  await clusterStore.add(cluster.uri);

  const state = clusterStore.getState();
  expect(state.get(cluster.uri)).toEqual(cluster);
});

test('adding a cluster does not overwrite an existing one', async () => {
  const mockClient = new MockTshClient();
  mockClient.addCluster = () => new MockedUnaryCall(cluster);
  mockClient.getCluster = () => new MockedUnaryCall(clusterWithDetails);
  const clusterStore = new ClusterStore(
    () => Promise.resolve(mockClient),
    mockWindowsManager
  );

  await clusterStore.sync(cluster.uri);
  // addCluster call returns fewer details than getCluster,
  // so clusterStore.add shouldn't overwrite details already acquired
  // by clusterStore.sync.
  await clusterStore.add(cluster.uri);

  const state = clusterStore.getState();
  expect(state.get(cluster.uri)).toEqual(clusterWithDetails);
});

test('syncs cluster', async () => {
  const mockClient = new MockTshClient();
  mockClient.getCluster = () => new MockedUnaryCall(clusterWithDetails);
  mockClient.listLeafClusters = () =>
    new MockedUnaryCall({ clusters: [leafCluster] });
  const clusterStore = new ClusterStore(
    () => Promise.resolve(mockClient),
    mockWindowsManager
  );

  await clusterStore.sync(cluster.uri);

  const state = clusterStore.getState();
  expect(state.get(clusterWithDetails.uri)).toStrictEqual(clusterWithDetails);
  expect(state.get(leafCluster.uri)).toStrictEqual(leafCluster);
});

test('logs out of cluster', async () => {
  const mockClient = new MockTshClient();
  mockClient.getCluster = () => new MockedUnaryCall(clusterWithDetails);
  mockClient.listLeafClusters = () =>
    new MockedUnaryCall({ clusters: [leafCluster] });
  const logoutMock = jest.spyOn(mockClient, 'logout');
  const removeClusterMock = jest.spyOn(mockClient, 'removeCluster');
  const clusterStore = new ClusterStore(
    () => Promise.resolve(mockClient),
    mockWindowsManager
  );
  await clusterStore.sync(cluster.uri);

  await clusterStore.logout(cluster.uri);

  expect(logoutMock).toHaveBeenCalledWith({ clusterUri: cluster.uri });
  expect(removeClusterMock).toHaveBeenCalledWith({
    clusterUri: cluster.uri,
  });
  const state = clusterStore.getState();
  expect(state.get(clusterWithDetails.uri)).toBeUndefined();
  expect(state.get(leafCluster.uri)).toBeUndefined();
});
