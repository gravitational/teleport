import { WorkspacesService } from '../workspacesService';
import { ClustersService } from '../clusters';
import { StatePersistenceService } from '../statePersistence';

import { ConnectionTrackerService } from './connectionTrackerService';
import { TrackedConnection } from './types';

jest.mock('../workspacesService');
jest.mock('../clusters');
jest.mock('../statePersistence');

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
    new ClustersService(undefined, undefined)
  );
}

it('removeItemsBelongingToRootCluster removes connections', () => {
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
