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

import { MainProcessClient } from 'teleterm/mainProcess/types';
import type { TshdClient } from 'teleterm/services/tshd';
import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import {
  makeDatabaseGateway,
  makeKubeGateway,
  makeLeafCluster,
  makeRootCluster,
} from 'teleterm/services/tshd/testHelpers';
import { NotificationsService } from 'teleterm/ui/services/notifications';
import { UsageService } from 'teleterm/ui/services/usage';
import type * as uri from 'teleterm/ui/uri';

import { ClustersService } from './clustersService';

jest.mock('teleterm/ui/services/notifications');
jest.mock('teleterm/ui/services/usage');

const clusterUri: uri.RootClusterUri = '/clusters/test';

const clusterMock = makeRootCluster({
  uri: clusterUri,
  name: 'Test',
  proxyHost: 'localhost:3080',
});

const leafClusterMock = makeLeafCluster({
  uri: `${clusterUri}/leaves/test2`,
  name: 'Leaf',
});

const gatewayMock = makeDatabaseGateway({
  uri: '/gateways/gatewayTestUri',
  targetUri: `${clusterUri}/dbs/databaseTestUri`,
});

const NotificationsServiceMock = NotificationsService as jest.MockedClass<
  typeof NotificationsService
>;
const UsageServiceMock = UsageService as jest.MockedClass<typeof UsageService>;

function createService(client: Partial<TshdClient>): ClustersService {
  return new ClustersService(
    client as TshdClient,
    {
      removeKubeConfig: jest.fn().mockResolvedValueOnce(undefined),
    } as unknown as MainProcessClient,
    new NotificationsServiceMock(),
    new UsageServiceMock(undefined, undefined, undefined, undefined, undefined)
  );
}

function getClientMocks(): Partial<TshdClient> {
  return {
    login: jest.fn().mockReturnValueOnce(new MockedUnaryCall({})),
    logout: jest.fn().mockReturnValueOnce(new MockedUnaryCall({})),
    addCluster: jest.fn().mockReturnValueOnce(new MockedUnaryCall(clusterMock)),
    removeCluster: jest.fn().mockReturnValueOnce(new MockedUnaryCall({})),
    getCluster: jest.fn().mockReturnValueOnce(new MockedUnaryCall(clusterMock)),
    listLeafClusters: jest
      .fn()
      .mockReturnValueOnce(
        new MockedUnaryCall({ clusters: [leafClusterMock] })
      ),
    listGateways: jest
      .fn()
      .mockReturnValueOnce(new MockedUnaryCall({ gateways: [gatewayMock] })),
    createGateway: jest
      .fn()
      .mockReturnValueOnce(new MockedUnaryCall(gatewayMock)),
    removeGateway: jest.fn().mockReturnValueOnce(new MockedUnaryCall({})),
    startHeadlessWatcher: jest
      .fn()
      .mockReturnValueOnce(new MockedUnaryCall({})),
  };
}

test('add cluster', async () => {
  const { addCluster } = getClientMocks();
  const service = createService({
    addCluster,
  });

  await service.addRootCluster(clusterUri);

  expect(addCluster).toHaveBeenCalledWith({ name: clusterUri });
  expect(service.state.clusters).toStrictEqual(
    new Map([[clusterUri, clusterMock]])
  );
});

test('add cluster does not overwrite the existing cluster', async () => {
  const { addCluster } = getClientMocks();
  const service = createService({
    addCluster,
  });
  service.state.clusters.set(clusterUri, {
    ...clusterMock,
    features: { advancedAccessWorkflows: true, isUsageBasedBilling: true },
  });

  await service.addRootCluster(clusterUri);

  expect(addCluster).toHaveBeenCalledWith({ name: clusterUri });
  expect(service.state.clusters).toStrictEqual(
    new Map([
      [
        clusterUri,
        {
          ...clusterMock,
          features: {
            advancedAccessWorkflows: true,
            isUsageBasedBilling: true,
          },
        },
      ],
    ])
  );
});

test('remove cluster', async () => {
  const { removeGateway } = getClientMocks();
  const service = createService({ removeGateway });
  const gatewayFromRootCluster = makeDatabaseGateway({
    uri: '/gateways/1',
    targetUri: `${clusterMock.uri}/dbs/foo`,
  });
  const gatewayFromLeafCluster = makeDatabaseGateway({
    uri: '/gateways/2',
    targetUri: `${leafClusterMock.uri}/dbs/foo`,
  });
  const gatewayFromOtherCluster = makeDatabaseGateway({
    uri: '/gateways/3',
    targetUri: `/clusters/bogus-cluster/dbs/foo`,
  });

  service.setState(draftState => {
    draftState.clusters = new Map([
      [clusterMock.uri, clusterMock],
      [leafClusterMock.uri, leafClusterMock],
    ]);
    draftState.gateways = new Map([
      [gatewayFromRootCluster.uri, gatewayFromRootCluster],
      [gatewayFromLeafCluster.uri, gatewayFromLeafCluster],
      [gatewayFromOtherCluster.uri, gatewayFromOtherCluster],
    ]);
  });

  await service.removeClusterAndResources(clusterUri);

  expect(service.findCluster(clusterUri)).toBeUndefined();
  expect(service.findCluster(leafClusterMock.uri)).toBeUndefined();
  expect(service.state.gateways).toEqual(
    new Map([[gatewayFromOtherCluster.uri, gatewayFromOtherCluster]])
  );

  expect(removeGateway).toHaveBeenCalledWith({
    gatewayUri: gatewayFromRootCluster.uri,
  });
  expect(removeGateway).toHaveBeenCalledWith({
    gatewayUri: gatewayFromLeafCluster.uri,
  });
  expect(removeGateway).not.toHaveBeenCalledWith({
    gatewayUri: gatewayFromOtherCluster.uri,
  });
});

test('sync root cluster', async () => {
  const { getCluster, listLeafClusters, startHeadlessWatcher } =
    getClientMocks();
  const service = createService({
    getCluster,
    listLeafClusters,
    startHeadlessWatcher,
  });

  await service.syncAndWatchRootClusterWithErrorHandling(clusterUri);

  const clusterMockWithRequests = {
    ...clusterMock,
    loggedInUser: { ...clusterMock.loggedInUser, assumedRequests: {} },
  };
  expect(service.findCluster(clusterUri)).toStrictEqual(
    clusterMockWithRequests
  );
  expect(service.findCluster(leafClusterMock.uri)).toStrictEqual(
    leafClusterMock
  );
  expect(listLeafClusters).toHaveBeenCalledWith({ clusterUri });
  expect(startHeadlessWatcher).toHaveBeenCalledWith({
    rootClusterUri: clusterUri,
  });
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

  expect(client.login).toHaveBeenCalledWith(
    {
      clusterUri: loginParams.clusterUri,
      params: {
        oneofKind: 'local',
        local: {
          password: loginParams.password,
          user: loginParams.username,
          token: loginParams.token,
        },
      },
    },
    { abort: undefined }
  );
  expect(service.findCluster(clusterUri).connected).toBe(true);
});

test('logout from cluster', async () => {
  const { logout, removeCluster } = getClientMocks();
  const service = createService({
    logout,
    removeCluster,
    getCluster: () => new MockedUnaryCall({ ...clusterMock, connected: false }),
  });
  service.setState(draftState => {
    draftState.clusters = new Map([
      [clusterMock.uri, clusterMock],
      [leafClusterMock.uri, leafClusterMock],
    ]);
  });

  await service.logout(clusterUri);

  expect(logout).toHaveBeenCalledWith({ clusterUri });
  expect(removeCluster).toHaveBeenCalledWith({ clusterUri });
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

  await service.createGateway({
    targetUri,
    localPort: port,
    targetUser: user,
    targetSubresourceName: '',
  });

  expect(createGateway).toHaveBeenCalledWith({
    targetUri,
    localPort: port,
    targetUser: user,
    targetSubresourceName: '',
  });
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

  expect(removeGateway).toHaveBeenCalledWith({ gatewayUri });
  expect(service.findGateway(gatewayUri)).toBeUndefined();
});

test('remove a kube gateway', async () => {
  const { removeGateway } = getClientMocks();
  const service = createService({
    removeGateway,
  });
  const kubeGatewayMock = makeKubeGateway({
    uri: '/gateways/gatewayTestUri',
    targetUri: `${clusterUri}/kubes/testKubeId`,
  });

  service.setState(draftState => {
    draftState.gateways = new Map([[kubeGatewayMock.uri, kubeGatewayMock]]);
  });

  await service.removeKubeGateway(kubeGatewayMock.targetUri as uri.KubeUri);
  expect(removeGateway).toHaveBeenCalledTimes(1);
  expect(removeGateway).toHaveBeenCalledWith({
    gatewayUri: kubeGatewayMock.uri,
  });
  expect(service.findGateway(kubeGatewayMock.uri)).toBeUndefined();

  // Calling it again should not increase mock calls.
  await service.removeKubeGateway(kubeGatewayMock.targetUri as uri.KubeUri);
  expect(removeGateway).toHaveBeenCalledTimes(1);
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
  expect(listGateways).toHaveBeenCalledWith({});
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
