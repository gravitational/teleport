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

import { NotificationsService } from 'teleterm/ui/services/notifications';
import { UsageService } from 'teleterm/ui/services/usage';
import { MainProcessClient } from 'teleterm/mainProcess/types';
import { makeGateway } from 'teleterm/services/tshd/testHelpers';

import { ClustersService } from './clustersService';

import type * as uri from 'teleterm/ui/uri';
import type * as tsh from 'teleterm/services/tshd/types';

jest.mock('teleterm/ui/services/notifications');
jest.mock('teleterm/ui/services/usage');

const clusterUri: uri.RootClusterUri = '/clusters/test';

const clusterMock: tsh.Cluster = {
  uri: clusterUri,
  name: 'Test',
  connected: true,
  leaf: false,
  proxyHost: 'localhost:3080',
  authClusterId: '73c4746b-d956-4f16-9848-4e3469f70762',
  loggedInUser: {
    activeRequestsList: [],
    assumedRequests: {},
    name: 'admin',
    acl: {},
    sshLoginsList: [],
    rolesList: [],
    requestableRolesList: [],
    suggestedReviewersList: [],
  },
};

const leafClusterMock: tsh.Cluster = {
  uri: `${clusterUri}/leaves/test2`,
  name: 'Leaf',
  connected: true,
  leaf: true,
  proxyHost: 'localhost:3085',
  authClusterId: '98dc94c8-c9a0-40e7-9a09-016cde91c652',
  loggedInUser: {
    activeRequestsList: [],
    assumedRequests: {},
    name: 'admin',
    acl: {},
    sshLoginsList: [],
    rolesList: [],
    requestableRolesList: [],
    suggestedReviewersList: [],
  },
};

const gatewayMock = makeGateway({
  uri: '/gateways/gatewayTestUri',
  targetUri: `${clusterUri}/dbs/databaseTestUri`,
});

const NotificationsServiceMock = NotificationsService as jest.MockedClass<
  typeof NotificationsService
>;
const UsageServiceMock = UsageService as jest.MockedClass<typeof UsageService>;

function createService(client: Partial<tsh.TshClient>): ClustersService {
  return new ClustersService(
    client as tsh.TshClient,
    {
      removeKubeConfig: jest.fn().mockResolvedValueOnce(undefined),
    } as unknown as MainProcessClient,
    new NotificationsServiceMock(),
    new UsageServiceMock(undefined, undefined, undefined, undefined, undefined)
  );
}

function getClientMocks(): Partial<tsh.TshClient> {
  return {
    loginLocal: jest.fn().mockResolvedValueOnce(undefined),
    logout: jest.fn().mockResolvedValueOnce(undefined),
    addRootCluster: jest.fn().mockResolvedValueOnce(clusterMock),
    removeCluster: jest.fn().mockResolvedValueOnce(undefined),
    getCluster: jest.fn().mockResolvedValueOnce(clusterMock),
    listLeafClusters: jest.fn().mockResolvedValueOnce([leafClusterMock]),
    listGateways: jest.fn().mockResolvedValueOnce([gatewayMock]),
    createGateway: jest.fn().mockResolvedValueOnce(gatewayMock),
    removeGateway: jest.fn().mockResolvedValueOnce(undefined),
  };
}

test('add cluster', async () => {
  const { addRootCluster } = getClientMocks();
  const service = createService({
    addRootCluster,
  });

  await service.addRootCluster(clusterUri);

  expect(addRootCluster).toHaveBeenCalledWith(clusterUri);
  expect(service.state.clusters).toStrictEqual(
    new Map([[clusterUri, clusterMock]])
  );
});

test('remove cluster', async () => {
  const service = createService({});

  service.setState(draftState => {
    draftState.clusters = new Map([
      [clusterMock.uri, clusterMock],
      [leafClusterMock.uri, leafClusterMock],
    ]);
  });

  await service.removeClusterAndResources(clusterUri);

  expect(service.findCluster(clusterUri)).toBeUndefined();
  expect(service.findCluster(leafClusterMock.uri)).toBeUndefined();
});

test('sync root cluster', async () => {
  const { getCluster, listLeafClusters } = getClientMocks();
  const service = createService({
    getCluster,
    listLeafClusters,
  });

  await service.syncRootClusterAndCatchErrors(clusterUri);

  expect(service.findCluster(clusterUri)).toStrictEqual(clusterMock);
  expect(service.findCluster(leafClusterMock.uri)).toStrictEqual(
    leafClusterMock
  );
  expect(listLeafClusters).toHaveBeenCalledWith(clusterUri);
});

test('login into cluster and sync cluster', async () => {
  const client = getClientMocks();
  const service = createService(client);
  const loginParams = {
    kind: 'local' as const,
    clusterUri,
    username: 'admin',
    password: 'admin',
    token: '1234',
  };

  await service.loginLocal(loginParams, undefined);

  expect(client.loginLocal).toHaveBeenCalledWith(loginParams, undefined);
  expect(service.findCluster(clusterUri).connected).toBe(true);
});

test('logout from cluster', async () => {
  const { logout, removeCluster } = getClientMocks();
  const service = createService({
    logout,
    removeCluster,
    getCluster: () => Promise.resolve({ ...clusterMock, connected: false }),
  });
  service.setState(draftState => {
    draftState.clusters = new Map([
      [clusterMock.uri, clusterMock],
      [leafClusterMock.uri, leafClusterMock],
    ]);
  });

  await service.logout(clusterUri);

  expect(logout).toHaveBeenCalledWith(clusterUri);
  expect(removeCluster).toHaveBeenCalledWith(clusterUri);
  expect(service.findCluster(clusterMock.uri).connected).toBe(false);
  expect(service.findCluster(leafClusterMock.uri).connected).toBe(false);
});

test('create a gateway', async () => {
  const { createGateway } = getClientMocks();
  const service = createService({
    createGateway,
  });
  const targetUri = '/clusters/foo/dbs/testId';
  const port = '2000';
  const user = 'alice';

  await service.createGateway({ targetUri, port, user });

  expect(createGateway).toHaveBeenCalledWith({ targetUri, port, user });
  expect(service.state.gateways).toStrictEqual(
    new Map([[gatewayMock.uri, gatewayMock]])
  );
});

test('remove a gateway', async () => {
  const { removeGateway } = getClientMocks();
  const service = createService({
    removeGateway,
  });
  const gatewayUri = '/gateways/gatewayUri';

  await service.removeGateway(gatewayUri);

  expect(removeGateway).toHaveBeenCalledWith(gatewayUri);
  expect(service.findGateway(gatewayUri)).toBeUndefined();
});

test('sync gateways', async () => {
  const { listGateways } = getClientMocks();
  const service = createService({
    listGateways,
  });

  await service.syncGatewaysAndCatchErrors();

  expect(service.state.gateways).toStrictEqual(
    new Map([[gatewayMock.uri, gatewayMock]])
  );
  expect(listGateways).toHaveBeenCalledWith();
});

test('find root cluster by resource URI', () => {
  const service = createService({});
  service.setState(draftState => {
    draftState.clusters = new Map([
      [clusterMock.uri, clusterMock],
      [leafClusterMock.uri, leafClusterMock],
    ]);
  });

  const foundClusters = service.findClusterByResource(
    `${clusterUri}/servers/foo`
  );

  expect(foundClusters).toStrictEqual(clusterMock);
});

test('find leaf cluster by resource URI', () => {
  const service = createService({});
  service.setState(draftState => {
    draftState.clusters = new Map([
      [clusterMock.uri, clusterMock],
      [leafClusterMock.uri, leafClusterMock],
    ]);
  });

  const foundClusters = service.findClusterByResource(
    `${leafClusterMock.uri}/servers/foo`
  );

  expect(foundClusters).toStrictEqual(leafClusterMock);
});
