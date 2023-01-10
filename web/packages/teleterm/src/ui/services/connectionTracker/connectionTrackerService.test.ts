import Logger, { NullService } from 'teleterm/logger';

import { DocumentGateway, WorkspacesService } from '../workspacesService';
import { ClustersService } from '../clusters';
import { StatePersistenceService } from '../statePersistence';

import { getEmptyPendingAccessRequest } from '../workspacesService/accessRequestsService';

import { ConnectionTrackerService } from './connectionTrackerService';
import { TrackedConnection, TrackedGatewayConnection } from './types';

jest.mock('../clusters');
jest.mock('../statePersistence');

beforeAll(() => {
  Logger.init(new NullService());
});

afterEach(() => {
  jest.restoreAllMocks();
});

function getTestSetup({ connections }: { connections: TrackedConnection[] }) {
  jest
    .spyOn(StatePersistenceService.prototype, 'getConnectionTrackerState')
    .mockImplementation(() => {
      return {
        connections,
      };
    });

  return new ConnectionTrackerService(
    new StatePersistenceService(undefined),
    new WorkspacesService(undefined, undefined, undefined, undefined),
    new ClustersService(undefined, undefined, undefined, undefined)
  );
}

it('removeItemsBelongingToRootCluster removes connections', () => {
  jest.mock('../workspacesService');

  const connections: TrackedConnection[] = [
    {
      kind: 'connection.server',
      connected: true,
      id: 'acwwOZvjJmj_sKohd9QYj',
      title: 'alice@teleport_root',
      login: 'alice',
      serverUri:
        '/clusters/localhost/servers/189dbe6d-dfdd-4e9f-8063-de5446f750db',
    },
    {
      kind: 'connection.server',
      connected: false,
      id: 'ni59F6wNjii5u9AXGguJ7',
      title: 'alice@teleport_leaf',
      login: 'alice',
      serverUri:
        '/clusters/localhost/leaves/teleport_leaf/servers/41f2ae3c-e96d-498d-b555-64ddd34c4a34',
    },
    {
      kind: 'connection.gateway',
      connected: true,
      id: 'Swm-6IgVvKGeUSSAJ0irc',
      title: 'alice@test/pg',
      port: '49671',
      targetUri: '/clusters/localhost/dbs/test',
      targetUser: 'alice',
      targetName: 'test',
      targetSubresourceName: 'pg',
      gatewayUri: '/gateways/4f68927b-579c-47a8-b965-efa8159203c9',
    },
    {
      kind: 'connection.server',
      connected: true,
      id: 'qT6-nUuDlGEk6kVmnMvt8',
      title: 'grzegorz@remote_cluster',
      login: 'grzegorz',
      serverUri:
        '/clusters/remote_cluster/leaves/remote_leaf/servers/51f23e3c-e96d-498e-b555-84ddd34c4a36',
    },
  ];

  const service = getTestSetup({ connections });
  service.removeItemsBelongingToRootCluster('/clusters/localhost');
  expect(service.getConnections()).toEqual([
    { clusterName: 'remote_leaf', ...connections[3] },
  ]);
});

it('updates the port of a gateway connection when the underlying doc gets updated', () => {
  const StatePersistenceServiceMock =
    StatePersistenceService as jest.MockedClass<typeof StatePersistenceService>;
  const ClustersServiceMock = ClustersService as jest.MockedClass<
    typeof ClustersService
  >;

  const mockedStatePersistenceService = new StatePersistenceServiceMock(
    undefined
  );
  jest
    .spyOn(mockedStatePersistenceService, 'getConnectionTrackerState')
    .mockImplementation(() => {
      return {
        connections: [],
      };
    });

  const workspacesService = new WorkspacesService(
    undefined,
    undefined,
    undefined,
    mockedStatePersistenceService
  );
  const connectionTrackerService = new ConnectionTrackerService(
    mockedStatePersistenceService,
    workspacesService,
    new ClustersServiceMock(undefined, undefined, undefined, undefined)
  );

  const document: DocumentGateway = {
    kind: 'doc.gateway',
    uri: '/docs/test-doc-uri',
    title: 'Test title',
    gatewayUri: '/gateways/4f68927b-579c-47a8-b965-efa8159203c9',
    targetUri: '/clusters/localhost/dbs/test',
    targetUser: 'alice',
    targetName: 'test',
    targetSubresourceName: 'pg',
    port: '12345',
  };

  // Insert the document.
  workspacesService.setState(draftState => {
    draftState.workspaces['/clusters/localhost'] = {
      accessRequests: {
        pending: getEmptyPendingAccessRequest(),
        isBarCollapsed: false,
      },
      localClusterUri: '/clusters/localhost',
      location: document.uri,
      documents: [document],
    };
  });

  let connection = connectionTrackerService.findConnectionByDocument(document);

  expect(connection.kind).toBe('connection.gateway');
  expect((connection as TrackedGatewayConnection).port).toBe('12345');

  // Update the document.
  workspacesService.setState(draftState => {
    const doc = draftState.workspaces['/clusters/localhost'].documents[0];
    if (doc.kind === 'doc.gateway') {
      doc.port = '54321';
    } else {
      throw new Error('Expected doc to be doc.gateway');
    }
  });

  connection = connectionTrackerService.findConnectionByDocument(document);

  expect(connection.kind).toBe('connection.gateway');
  expect((connection as TrackedGatewayConnection).port).toBe('54321');
});
