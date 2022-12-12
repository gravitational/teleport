import { tsh, SyncStatus } from 'teleterm/ui/services/clusters/types';

import { NotificationsService } from 'teleterm/ui/services/notifications';
import { MainProcessClient } from 'teleterm/mainProcess/types';

import { ClustersService } from './clustersService';

const clusterUri = '/clusters/test';

const clusterMock: tsh.Cluster = {
  uri: clusterUri,
  name: 'Test',
  connected: true,
  leaf: false,
  proxyHost: 'localhost:3080',
  loggedInUser: {
    activeRequestsList: [],
    assumedRequests: {},
    name: 'admin',
    acl: {},
    sshLoginsList: [],
    rolesList: [],
  },
};

const dbMock: tsh.Database = {
  uri: `${clusterUri}/dbs/databaseTestUri`,
  desc: 'Desc',
  name: 'Name',
  addr: 'addr',
  protocol: 'psql',
  type: '',
  hostname: 'localhost',
  labelsList: [{ name: 'type', value: 'postgres' }],
};

const gatewayMock: tsh.Gateway = {
  uri: 'gatewayTestUri',
  localAddress: 'localhost',
  localPort: '2000',
  protocol: 'https',
  targetName: dbMock.name,
  targetSubresourceName: '',
  targetUser: 'sam',
  targetUri: dbMock.uri,
  cliCommand: 'psql postgres://postgres@localhost:5432/postgres',
};

const serverMock: tsh.Server = {
  uri: `${clusterUri}/servers/serverTestUri`,
  addr: 'addr',
  name: 'Name',
  hostname: 'localhost',
  labelsList: [
    {
      name: 'Type',
      value: 'Unknown',
    },
  ],
  tunnel: true,
};

const kubeMock: tsh.Kube = {
  uri: `${clusterUri}/kubes/kubeTestUri`,
  name: 'TestKube',
  labelsList: [
    {
      name: 'Type',
      value: 'K8',
    },
  ],
};

const appMock: tsh.Application = {
  uri: `${clusterUri}/apps/appTestUri`,
  name: 'TestApp',
  labelsList: [
    {
      name: 'Type',
      value: 'OnDemand',
    },
  ],
  appUri: 'appTestUri',
  awsConsole: false,
  awsRolesList: [],
  description: '',
  fqdn: '',
  publicAddr: 'app.test',
};

const NotificationsServiceMock = NotificationsService as jest.MockedClass<
  typeof NotificationsService
>;

function createService(
  client: Partial<tsh.TshClient>,
  notificationsService?: NotificationsService
): ClustersService {
  return new ClustersService(
    client as tsh.TshClient,
    {
      removeKubeConfig: jest.fn().mockResolvedValueOnce(undefined),
    } as unknown as MainProcessClient,
    notificationsService
  );
}

function getClientMocks(): Partial<tsh.TshClient> {
  return {
    loginLocal: jest.fn().mockResolvedValueOnce(undefined),
    loginSso: jest.fn().mockResolvedValueOnce(undefined),
    loginPasswordless: jest.fn().mockResolvedValueOnce(undefined),
    logout: jest.fn().mockResolvedValueOnce(undefined),
    addRootCluster: jest.fn().mockResolvedValueOnce(clusterMock),
    removeCluster: jest.fn().mockResolvedValueOnce(undefined),
    getCluster: jest.fn().mockResolvedValueOnce(clusterMock),
    listLeafClusters: jest.fn().mockResolvedValueOnce([]),
    listGateways: jest.fn().mockResolvedValueOnce([gatewayMock]),
    getAllDatabases: jest.fn().mockResolvedValueOnce([dbMock]),
    getAllServers: jest.fn().mockResolvedValueOnce([serverMock]),
    createGateway: jest.fn().mockResolvedValueOnce(gatewayMock),
    removeGateway: jest.fn().mockResolvedValueOnce(undefined),
  };
}

function testIfClusterResourcesHaveBeenCleared(service: ClustersService): void {
  expect(service.findServers(clusterUri)).toStrictEqual([]);
  expect(service.findDbs(clusterUri)).toStrictEqual([]);
  expect(service.getClusterSyncStatus(clusterUri)).toStrictEqual({
    syncing: false,
    dbs: { status: '' },
    servers: { status: '' },
    apps: { status: '' },
    kubes: { status: '' },
  });
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
  const { removeCluster } = getClientMocks();
  const service = createService({
    removeCluster,
  });

  await service.removeCluster(clusterUri);

  expect(removeCluster).toHaveBeenCalledWith(clusterUri);
  testIfClusterResourcesHaveBeenCleared(service);
});

test('sync cluster and its resources', async () => {
  const {
    getCluster,
    listLeafClusters,
    listGateways,
    getAllDatabases,
    getAllServers,
  } = getClientMocks();
  const service = createService({
    getCluster,
    listLeafClusters,
    listGateways,
    getAllDatabases,
    getAllServers,
  });

  await service.syncRootCluster(clusterUri);

  expect(service.findCluster(clusterUri)).toStrictEqual(clusterMock);
  expect(listGateways).toHaveBeenCalledWith();
  expect(getAllDatabases).toHaveBeenCalledWith(clusterUri);
  expect(getAllServers).toHaveBeenCalledWith(clusterUri);
});

test('login into cluster and sync resources', async () => {
  const client = getClientMocks();
  const service = createService(client, new NotificationsServiceMock());
  const loginParams = {
    kind: 'local' as const,
    clusterUri,
    username: 'admin',
    password: 'admin',
    token: '1234',
  };

  // Add mocked gateway to service state.
  await service.syncGateways();

  await service.loginLocal(loginParams, undefined);

  expect(client.loginLocal).toHaveBeenCalledWith(loginParams, undefined);
  expect(client.listGateways).toHaveBeenCalledWith();
  expect(client.getAllDatabases).toHaveBeenCalledWith(clusterUri);
  expect(client.getAllServers).toHaveBeenCalledWith(clusterUri);
  expect(service.findCluster(clusterUri).connected).toBe(true);
});

test('logout from cluster and clean its resources', async () => {
  const { logout, removeCluster } = getClientMocks();
  const service = createService({
    logout,
    removeCluster,
    getCluster: () => Promise.resolve({ ...clusterMock, connected: false }),
  });
  service.setState(draftState => {
    draftState.clusters = new Map([[clusterMock.uri, clusterMock]]);
  });

  await service.logout(clusterUri);

  expect(logout).toHaveBeenCalledWith(clusterUri);
  expect(service.findCluster(clusterUri)).toBeUndefined();
  testIfClusterResourcesHaveBeenCleared(service);
});

test('create a gateway', async () => {
  const { createGateway } = getClientMocks();
  const service = createService({
    createGateway,
  });
  const targetUri = 'testId';
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
  const gatewayUri = 'gatewayUri';

  await service.removeGateway(gatewayUri);

  expect(removeGateway).toHaveBeenCalledWith(gatewayUri);
  expect(service.findGateway(gatewayUri)).toBeUndefined();
});

test('sync gateways', async () => {
  const { listGateways } = getClientMocks();
  const service = createService({
    listGateways,
  });

  await service.syncGateways();

  expect(service.getGateways()).toStrictEqual([gatewayMock]);
});

test('sync databases', async () => {
  const { getAllDatabases } = getClientMocks();
  const service = createService({
    getAllDatabases,
  });
  service.setState(draftState => {
    draftState.clusters.set(clusterUri, clusterMock);
  });

  await service.syncDbs(clusterUri);

  expect(getAllDatabases).toHaveBeenCalledWith(clusterUri);
  expect(service.getDbs()).toStrictEqual([dbMock]);
  const readySyncStatus: SyncStatus = { status: 'ready' };
  expect(service.getClusterSyncStatus(clusterUri).dbs).toStrictEqual(
    readySyncStatus
  );
});

test('sync servers', async () => {
  const { getAllServers } = getClientMocks();
  const service = createService({
    getAllServers,
  });
  service.setState(draftState => {
    draftState.clusters.set(clusterUri, clusterMock);
  });

  await service.syncServers(clusterUri);

  expect(getAllServers).toHaveBeenCalledWith(clusterUri);
  expect(service.getServers()).toStrictEqual([serverMock]);
  const readySyncStatus: SyncStatus = { status: 'ready' };
  expect(service.getClusterSyncStatus(clusterUri).servers).toStrictEqual(
    readySyncStatus
  );
});

test('find servers by cluster uri', () => {
  const service = createService({});
  service.setState(draftState => {
    draftState.servers.set(serverMock.uri, serverMock);
  });

  const foundServers = service.findServers(clusterUri);
  expect(foundServers).toStrictEqual([serverMock]);
});

test('find databases by cluster uri', () => {
  const service = createService({});
  service.setState(draftState => {
    draftState.dbs.set(dbMock.uri, dbMock);
  });

  const foundDbs = service.findDbs(clusterUri);

  expect(foundDbs).toStrictEqual([dbMock]);
});

test('find kubes by cluster uri', () => {
  const service = createService({});
  service.setState(draftState => {
    draftState.kubes.set(kubeMock.uri, kubeMock);
  });

  const foundKubes = service.findKubes(clusterUri);

  expect(foundKubes).toStrictEqual([kubeMock]);
});

test('find apps by cluster uri', () => {
  const service = createService({});
  service.setState(draftState => {
    draftState.apps.set(appMock.uri, appMock);
  });

  const foundApps = service.findApps(clusterUri);

  expect(foundApps).toStrictEqual([appMock]);
});

test('find cluster by resource uri', () => {
  const service = createService({});
  service.setState(draftState => {
    draftState.clusters.set(clusterUri, clusterMock);
  });

  const foundClusters = service.findClusterByResource(
    `${clusterUri}/ae321-dkf32`
  );

  expect(foundClusters).toStrictEqual(clusterMock);
});

test.each([
  { prop: 'name', value: dbMock.name },
  { prop: 'desc', value: dbMock.desc },
  { prop: 'labelsList', value: dbMock.labelsList[0].value },
])('search dbs by prop: $prop', ({ value }) => {
  const service = createService({});
  service.setState(draftState => {
    draftState.dbs.set(dbMock.uri, dbMock);
  });

  const foundDbs = service.searchDbs(clusterUri, {
    search: value.toLocaleLowerCase(),
  });

  expect(foundDbs).toStrictEqual([dbMock]);
});

test.each([
  { prop: 'name', value: appMock.name },
  { prop: 'publicAddr', value: appMock.publicAddr },
  { prop: 'description', value: appMock.description },
  { prop: 'labelsList', value: appMock.labelsList[0].value },
])('search apps by prop: $prop', ({ value }) => {
  const service = createService({});
  service.setState(draftState => {
    draftState.apps.set(appMock.uri, appMock);
  });

  const foundApps = service.searchApps(clusterUri, {
    search: value.toLocaleLowerCase(),
  });

  expect(foundApps).toStrictEqual([appMock]);
});

test.each([
  { prop: 'name', value: kubeMock.name },
  { prop: 'labelsList', value: kubeMock.labelsList[0].value },
])('search kubes by prop: $prop', ({ value }) => {
  const service = createService({});
  service.setState(draftState => {
    draftState.kubes.set(kubeMock.uri, kubeMock);
  });

  const foundKubes = service.searchKubes(clusterUri, {
    search: value.toLocaleLowerCase(),
  });

  expect(foundKubes).toStrictEqual([kubeMock]);
});

test.each([
  { prop: 'hostname', value: serverMock.hostname },
  { prop: 'addr', value: serverMock.addr },
  { prop: 'tunnel', value: 'TUNNEL' },
  {
    prop: 'labelsList',
    value: serverMock.labelsList[0].value,
  },
])('search servers by prop: $prop', ({ value }) => {
  const service = createService({});
  service.setState(draftState => {
    draftState.servers.set(serverMock.uri, serverMock);
  });

  const foundServers = service.searchServers(clusterUri, {
    search: value.toLocaleLowerCase(),
  });

  expect(foundServers).toStrictEqual([serverMock]);
});
