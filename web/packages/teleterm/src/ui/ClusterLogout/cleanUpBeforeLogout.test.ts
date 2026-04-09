/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import {
  makeDatabaseGateway,
  makeRootCluster,
} from 'teleterm/services/tshd/testHelpers';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { TrackedServerConnection } from 'teleterm/ui/services/connectionTracker/types';
import { makeDocumentVnetInfo } from 'teleterm/ui/services/workspacesService/documentsService/testHelpers';

import { cleanUpBeforeLogout } from './cleanUpBeforeLogout';

function makeAppContext() {
  const appContext = new MockAppContext();
  // Initialize restoredState so that removeWorkspace/clearWorkspace work.
  appContext.workspacesService.restorePersistedState();
  return appContext;
}

it('switches to another connected workspace when the active one is being logged out', async () => {
  const clusterA = makeRootCluster({ uri: '/clusters/cluster-a' });
  const clusterB = makeRootCluster({ uri: '/clusters/cluster-b' });
  const appContext = makeAppContext();
  appContext.addRootCluster(clusterA, { noActivate: true });
  appContext.addRootCluster(clusterB);

  expect(appContext.workspacesService.getRootClusterUri()).toBe(clusterB.uri);

  await cleanUpBeforeLogout(appContext, clusterB.uri, {
    removeWorkspace: false,
  });

  expect(appContext.workspacesService.getRootClusterUri()).toBe(clusterA.uri);
});

it('sets active workspace to null when no other connected workspaces exist', async () => {
  const cluster = makeRootCluster();
  const appContext = makeAppContext();
  appContext.addRootCluster(cluster);

  await cleanUpBeforeLogout(appContext, cluster.uri, {
    removeWorkspace: false,
  });

  expect(appContext.workspacesService.getRootClusterUri()).toBeUndefined();
});

it('does not switch workspace when the logged out cluster is not the active one', async () => {
  const clusterA = makeRootCluster({ uri: '/clusters/cluster-a' });
  const clusterB = makeRootCluster({ uri: '/clusters/cluster-b' });
  const appContext = makeAppContext();
  appContext.addRootCluster(clusterA);
  appContext.addRootCluster(clusterB, { noActivate: true });

  expect(appContext.workspacesService.getRootClusterUri()).toBe(clusterA.uri);

  await cleanUpBeforeLogout(appContext, clusterB.uri, {
    removeWorkspace: false,
  });

  expect(appContext.workspacesService.getRootClusterUri()).toBe(clusterA.uri);
});

it('removes connections belonging to the cluster and keeps connections of other clusters', async () => {
  const clusterA = makeRootCluster({ uri: '/clusters/cluster-a' });
  const clusterB = makeRootCluster({ uri: '/clusters/cluster-b' });
  const appContext = makeAppContext();
  appContext.addRootCluster(clusterA);
  appContext.addRootCluster(clusterB, { noActivate: true });

  const connectionA: TrackedServerConnection = {
    kind: 'connection.server',
    connected: true,
    id: 'conn-a',
    title: 'server-a',
    login: 'root',
    serverUri: `${clusterA.uri}/servers/server-a`,
  };
  const connectionB: TrackedServerConnection = {
    kind: 'connection.server',
    connected: true,
    id: 'conn-b',
    title: 'server-b',
    login: 'root',
    serverUri: `${clusterB.uri}/servers/server-b`,
  };
  appContext.connectionTracker.setState(draft => {
    draft.connections = [connectionA, connectionB];
  });

  await cleanUpBeforeLogout(appContext, clusterA.uri, {
    removeWorkspace: false,
  });

  const conns = appContext.connectionTracker.getConnections();
  expect(conns).toHaveLength(1);
  expect(conns[0].id).toBe('conn-b');
});

it('removes workspace when removeWorkspace is true', async () => {
  const cluster = makeRootCluster();
  const appContext = makeAppContext();
  appContext.addRootCluster(cluster);

  expect(appContext.workspacesService.getWorkspace(cluster.uri)).toBeDefined();

  await cleanUpBeforeLogout(appContext, cluster.uri, {
    removeWorkspace: true,
  });

  expect(
    appContext.workspacesService.getWorkspace(cluster.uri)
  ).toBeUndefined();
});

it('clears workspace when removeWorkspace is false', async () => {
  const cluster = makeRootCluster();
  const appContext = makeAppContext();
  appContext.addRootCluster(cluster);

  // Add documents so we can verify the workspace gets reset to defaults.
  const docsService = appContext.workspacesService.getWorkspaceDocumentService(
    cluster.uri
  );
  docsService.add(
    makeDocumentVnetInfo({ uri: '/docs/vnet-1', rootClusterUri: cluster.uri })
  );
  docsService.add(
    makeDocumentVnetInfo({ uri: '/docs/vnet-2', rootClusterUri: cluster.uri })
  );
  expect(docsService.getDocuments()).toHaveLength(2);

  await cleanUpBeforeLogout(appContext, cluster.uri, {
    removeWorkspace: false,
  });

  const workspace = appContext.workspacesService.getWorkspace(cluster.uri);
  expect(workspace).toBeDefined();
  expect(workspace.documents).toHaveLength(1);
  expect(workspace.documents[0].kind).toBe('doc.cluster');
});

it('kills Connect My Computer agent and removes data', async () => {
  const cluster = makeRootCluster();
  const appContext = makeAppContext();
  appContext.addRootCluster(cluster);

  jest.spyOn(appContext.connectMyComputerService, 'killAgentAndRemoveData');

  await cleanUpBeforeLogout(appContext, cluster.uri, {
    removeWorkspace: false,
  });

  expect(
    appContext.connectMyComputerService.killAgentAndRemoveData
  ).toHaveBeenCalledWith(cluster.uri);
});

it('removes gateways belonging to the cluster', async () => {
  const cluster = makeRootCluster();
  const appContext = makeAppContext();
  appContext.addRootCluster(cluster);

  const gateway = makeDatabaseGateway({
    uri: '/gateways/gw1',
    targetUri: `${cluster.uri}/dbs/foo`,
  });
  appContext.clustersService.setState(draft => {
    draft.gateways.set(gateway.uri, gateway);
  });

  expect(appContext.clustersService.findGateway(gateway.uri)).toBeDefined();

  await cleanUpBeforeLogout(appContext, cluster.uri, {
    removeWorkspace: false,
  });

  expect(appContext.clustersService.findGateway(gateway.uri)).toBeUndefined();
});

it('performs cleanup steps in the correct order', async () => {
  const cluster = makeRootCluster();
  const appContext = makeAppContext();
  appContext.addRootCluster(cluster);

  const order: string[] = [];

  jest
    .spyOn(appContext.connectionTracker, 'removeItemsBelongingToRootCluster')
    .mockImplementation(() => {
      order.push('removeConnections');
    });
  jest
    .spyOn(appContext.workspacesService, 'removeWorkspace')
    .mockImplementation(() => {
      order.push('removeWorkspace');
    });
  jest
    .spyOn(appContext.connectMyComputerService, 'killAgentAndRemoveData')
    .mockImplementation(async () => {
      order.push('killAgent');
    });
  jest
    .spyOn(appContext.clustersService, 'removeClusterGateways')
    .mockImplementation(async () => {
      order.push('removeGateways');
    });

  await cleanUpBeforeLogout(appContext, cluster.uri, {
    removeWorkspace: true,
  });

  expect(order).toEqual([
    'removeConnections',
    'removeWorkspace',
    'killAgent',
    'removeGateways',
  ]);
});
