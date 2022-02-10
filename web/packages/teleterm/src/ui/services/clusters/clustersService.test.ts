import { tsh, SyncStatus } from 'teleterm/ui/services/clusters/types';
import { ClustersService } from './clustersService';

const clusterUri = 'testId';

const clusterMock: tsh.Cluster = {
  uri: clusterUri,
  name: 'Test',
  connected: true,
  leaf: false,
  loggedInUser: {
    name: 'admin',
    acl: {},
    sshLoginsList: [],
    rolesList: [],
  },
};

const gatewayMock: tsh.Gateway = {
  uri: 'gatewayTestUri',
  caCertPath: '',
  certPath: '',
  keyPath: '',
  localAddress: 'localhost',
  localPort: '2000',
  protocol: 'https',
  targetName: 'Test',
  targetUser: '',
  targetUri: 'clusters/xxx/',
  insecure: true,
};

const databaseMock: tsh.Database = {
  uri: '/clusters/test/dbs/databaseTestUri',
  desc: 'Desc',
  name: 'Name',
  addr: 'addr',
  clusterId: clusterUri,
  protocol: 'psql',
  type: '',
  hostname: 'localhost',
  labelsList: [],
};

const serverMock: tsh.Server = {
  uri: '/clusters/test/servers/serverTestUri',
  addr: 'addr',
  name: 'Name',
  hostname: 'localhost',
  labelsList: [],
  clusterId: clusterUri,
  tunnel: false,
};

function createService(client: Partial<tsh.TshClient>): ClustersService {
  return new ClustersService(client as tsh.TshClient);
}

function getClientMocks(): Partial<tsh.TshClient> {
  return {
    login: jest.fn().mockResolvedValueOnce(undefined),
    logout: jest.fn().mockResolvedValueOnce(undefined),
    addRootCluster: jest.fn().mockResolvedValueOnce(clusterMock),
    removeCluster: jest.fn().mockResolvedValueOnce(undefined),
    getCluster: jest.fn().mockResolvedValueOnce(clusterMock),
    listLeafClusters: jest.fn().mockResolvedValueOnce([]),
    listGateways: jest.fn().mockResolvedValueOnce([gatewayMock]),
    listDatabases: jest.fn().mockResolvedValueOnce([databaseMock]),
    listServers: jest.fn().mockResolvedValueOnce([serverMock]),
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
    listDatabases,
    listServers,
  } = getClientMocks();
  const service = createService({
    getCluster,
    listLeafClusters,
    listGateways,
    listDatabases,
    listServers,
  });

  await service.syncRootCluster(clusterUri);

  expect(service.findCluster(clusterUri)).toStrictEqual(clusterMock);
  expect(listGateways).toHaveBeenCalledWith();
  expect(listDatabases).toHaveBeenCalledWith(clusterUri);
  expect(listServers).toHaveBeenCalledWith(clusterUri);
});

test('login into cluster and sync resources', async () => {
  const {
    login,
    listLeafClusters,
    getCluster,
    listGateways,
    listDatabases,
    listServers,
  } = getClientMocks();
  const service = createService({
    login,
    listLeafClusters,
    getCluster,
    listGateways,
    listDatabases,
    listServers,
  });
  const loginParams = {
    clusterUri,
    local: { username: 'admin', password: 'admin', token: '1234' },
    oss: { providerType: 'github', providerName: 'GitHub' },
  };

  await service.login(loginParams, undefined);

  expect(login).toHaveBeenCalledWith(loginParams, undefined);
  expect(listGateways).toHaveBeenCalledWith();
  expect(listDatabases).toHaveBeenCalledWith(clusterUri);
  expect(listServers).toHaveBeenCalledWith(clusterUri);
  expect(service.findCluster(clusterUri).connected).toBe(true);
});

test('logout from cluster and clean its resources', async () => {
  const { logout } = getClientMocks();
  const service = createService({
    logout,
    getCluster: () => Promise.resolve({ ...clusterMock, connected: false }),
  });
  service.setState(draftState => {
    draftState.clusters = new Map([[clusterMock.uri, clusterMock]]);
  });

  await service.logout(clusterUri);

  expect(logout).toHaveBeenCalledWith(clusterUri);
  expect(service.findCluster(clusterUri).connected).toBe(false);
  testIfClusterResourcesHaveBeenCleared(service);
});

test('create a gateway', async () => {
  const { createGateway } = getClientMocks();
  const service = createService({
    createGateway,
  });
  const targetUri = 'testId';
  const port = '2000';

  await service.createGateway({ targetUri, port });

  expect(createGateway).toHaveBeenCalledWith({ targetUri, port });
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
  const { listDatabases } = getClientMocks();
  const service = createService({
    listDatabases,
  });
  service.setState(draftState => {
    draftState.clusters.set(clusterUri, clusterMock);
  });

  await service.syncDbs(clusterUri);

  expect(listDatabases).toHaveBeenCalledWith(clusterUri);
  expect(service.getDbs()).toStrictEqual([databaseMock]);
  const readySyncStatus: SyncStatus = { status: 'ready' };
  expect(service.getClusterSyncStatus(clusterUri).dbs).toStrictEqual(
    readySyncStatus
  );
});

test('sync servers', async () => {
  const { listServers } = getClientMocks();
  const service = createService({
    listServers,
  });
  service.setState(draftState => {
    draftState.clusters.set(clusterUri, clusterMock);
  });

  await service.syncServers(clusterUri);

  expect(listServers).toHaveBeenCalledWith(clusterUri);
  expect(service.getServers()).toStrictEqual([serverMock]);
  const readySyncStatus: SyncStatus = { status: 'ready' };
  expect(service.getClusterSyncStatus(clusterUri).servers).toStrictEqual(
    readySyncStatus
  );
});

test.skip('find servers by uri', () => {
  const service = createService({});
  service.setState(draftState => {
    draftState.servers.set(serverMock.uri, serverMock);
    draftState.servers.set('secondUri', { ...serverMock, uri: 'secondUri' });
  });

  const foundServers = service.findServers(serverMock.uri);
  expect(foundServers).toStrictEqual([serverMock]);
});

test.skip('find databases by uri', () => {
  const service = createService({});
  service.setState(draftState => {
    draftState.dbs.set(databaseMock.uri, databaseMock);
    draftState.dbs.set('secondUri', { ...databaseMock, uri: 'secondUri' });
  });

  const foundDbs = service.findDbs(databaseMock.uri);

  expect(foundDbs).toStrictEqual([databaseMock]);
});

test.skip('find cluster by resource uri', () => {
  const service = createService({});
  service.setState(draftState => {
    draftState.clusters.set(clusterUri, clusterMock);
    draftState.clusters.set('secondUri', { ...clusterMock, uri: 'secondUri' });
  });

  const foundClusters = service.findClusterByResource(
    `${clusterUri}/ae321-dkf32`
  );

  expect(foundClusters).toStrictEqual(clusterMock);
});
