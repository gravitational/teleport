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

import Logger, { NullService } from 'teleterm/logger';

import {
  Document,
  DocumentGateway,
  DocumentGatewayKube,
  DocumentTshNodeWithLoginHost,
  DocumentTshNodeWithServerId,
  WorkspacesService,
} from '../workspacesService';
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
