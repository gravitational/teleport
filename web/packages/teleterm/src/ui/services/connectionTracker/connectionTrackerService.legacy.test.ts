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

import Logger, { NullService } from 'teleterm/logger';

import { ClustersService } from '../clusters';
import { StatePersistenceService } from '../statePersistence';
import {
  Document,
  DocumentGateway,
  DocumentGatewayKube,
  DocumentTshNodeWithLoginHost,
  DocumentTshNodeWithServerId,
  WorkspacesService,
} from '../workspacesService';
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

// TODO(ravicious): Rewrite those tests to use MockAppContext instead of manually mocking everything.

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
    },
    {
      kind: 'connection.kube',
      connected: true,
      id: 'root-cluster-kube-id',
      title: 'test-kube-id',
      kubeUri: '/clusters/localhost/kubes/test-kube-id',
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

  const service = getTestSetupWithMockedConnections({ connections });
  service.removeItemsBelongingToRootCluster('/clusters/localhost');
  expect(service.getConnections()).toEqual([
    { clusterName: 'remote_leaf', ...connections[4] },
  ]);
});

it('updates the port of a gateway connection when the underlying doc gets updated', () => {
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
    origin: 'resource_table',
    status: '',
  };

  const { connectionTrackerService, workspacesService } =
    getTestSetupWithMockedDocuments([document]);

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

it('creates a connection for doc.terminal_tsh_node docs with serverUri', () => {
  const document: DocumentTshNodeWithServerId = {
    kind: 'doc.terminal_tsh_node',
    uri: '/docs/123',
    title: '',
    status: '',
    serverId: 'test',
    serverUri: '/clusters/localhost/servers/test',
    rootClusterId: 'localhost',
    leafClusterId: undefined,
    login: 'user',
    origin: 'resource_table',
  };

  const { connectionTrackerService } = getTestSetupWithMockedDocuments([
    document,
  ]);

  const connection =
    connectionTrackerService.findConnectionByDocument(document);
  expect(connection.connected).toBe(false);
  expect(connection).toEqual({
    kind: 'connection.server',
    id: expect.any(String),
    title: document.title,
    login: document.login,
    serverUri: document.serverUri,
    connected: false,
  });
});

it('ignores doc.terminal_tsh_node docs with no serverUri', () => {
  const document: DocumentTshNodeWithLoginHost = {
    kind: 'doc.terminal_tsh_node',
    uri: '/docs/123',
    title: '',
    status: '',
    loginHost: 'user@foo',
    rootClusterId: 'test',
    leafClusterId: undefined,
    origin: 'resource_table',
  };

  const { connectionTrackerService } = getTestSetupWithMockedDocuments([
    document,
  ]);

  expect(connectionTrackerService.getConnections()).toEqual([]);
});

it('creates a kube connection for doc.gateway_kube', () => {
  const document: DocumentGatewayKube = {
    kind: 'doc.gateway_kube',
    uri: '/docs/test-kube-id',
    title: 'Test title',
    rootClusterId: 'localhost',
    leafClusterId: undefined,
    targetUri: '/clusters/localhost/kubes/test-kube-id',
    origin: 'resource_table',
    status: '',
  };

  const { connectionTrackerService } = getTestSetupWithMockedDocuments([
    document,
  ]);

  const connection =
    connectionTrackerService.findConnectionByDocument(document);
  expect(connection).toEqual({
    kind: 'connection.kube',
    id: expect.any(String),
    title: document.title,
    connected: true,
    kubeUri: '/clusters/localhost/kubes/test-kube-id',
  });
});

function getTestSetupWithMockedConnections({
  connections,
}: {
  connections: TrackedConnection[];
}) {
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

function getTestSetupWithMockedDocuments(documents: Document[]) {
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

  // Insert the documents.
  workspacesService.setState(draftState => {
    draftState.workspaces['/clusters/localhost'] = {
      accessRequests: {
        pending: getEmptyPendingAccessRequest(),
        isBarCollapsed: false,
      },
      localClusterUri: '/clusters/localhost',
      location: documents[0]?.uri,
      documents: documents,
    };
  });

  return { workspacesService, connectionTrackerService };
}
