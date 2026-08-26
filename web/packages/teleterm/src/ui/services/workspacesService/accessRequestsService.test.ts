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

import {
  leafClusterUri,
  makeApp,
  makeDatabase,
  makeKube,
  makeServer,
  rootClusterUri,
} from 'teleterm/services/tshd/testHelpers';
import { ImmutableStore } from 'teleterm/ui/services/immutableStore';
import { ModalsService } from 'teleterm/ui/services/modals';

import {
  AccessRequestsService,
  getEmptyPendingAccessRequest,
  PendingAccessRequest,
} from './accessRequestsService';

function getMockPendingResourceAccessRequest(): PendingAccessRequest {
  const server = makeServer();
  const app1 = makeApp();
  const app2 = makeApp({ uri: `${rootClusterUri}/apps/foo2`, name: 'foo2' });
  const samlApp = makeApp({
    uri: `${rootClusterUri}/samlapp`,
    name: 'samlapp',
    samlApp: true,
  });
  const kube = makeKube();
  const database = makeDatabase();

  return {
    kind: 'resource',
    resources: new Map([
      [server.uri, { kind: 'server', resource: server }],
      [app1.uri, { kind: 'app', resource: app1 }],
      [app2.uri, { kind: 'app', resource: app2 }],
      [kube.uri, { kind: 'kube', resource: kube }],
      [database.uri, { kind: 'database', resource: database }],
      [samlApp.uri, { kind: 'app', resource: samlApp }],
    ]),
  };
}

function getMockPendingRoleAccessRequest(): PendingAccessRequest {
  return {
    kind: 'role',
    roles: new Set(['admin']),
  };
}

function getTestSetup(pending: PendingAccessRequest) {
  const store = new ImmutableStore<{
    isBarCollapsed: boolean;
    pending: PendingAccessRequest;
  }>();
  store.state = {
    isBarCollapsed: false,
    pending,
  };
  jest.mock('../modals');
  const ModalsServiceMock = ModalsService as jest.MockedClass<
    typeof ModalsService
  >;
  const modalsService = new ModalsServiceMock();
  return {
    accessRequestsService: new AccessRequestsService(
      modalsService,
      () => store.state,
      draftState => store.setState(draftState)
    ),
    modalsService,
  };
}

test('getCollapsed() returns the bar collapse state', () => {
  const { accessRequestsService: service } = getTestSetup(
    getMockPendingResourceAccessRequest()
  );
  expect(service.getCollapsed()).toBe(false);
});

test('toggleBar() changes the collapse state', () => {
  const { accessRequestsService: service } = getTestSetup(
    getMockPendingResourceAccessRequest()
  );
  expect(service.getCollapsed()).toBe(false);
  service.toggleBar();
  expect(service.getCollapsed()).toBe(true);
});

test('clearPendingAccessRequest() clears pending access request', () => {
  const { accessRequestsService: service } = getTestSetup(
    getMockPendingResourceAccessRequest()
  );
  service.clearPendingAccessRequest();
  expect(service.getPendingAccessRequest()).toStrictEqual(
    getEmptyPendingAccessRequest()
  );
});

test('getAddedItemsCount() returns added resource count for pending request', () => {
  const { accessRequestsService: service } = getTestSetup(
    getMockPendingResourceAccessRequest()
  );
  expect(service.getAddedItemsCount()).toBe(6);
  service.clearPendingAccessRequest();
  expect(service.getAddedItemsCount()).toBe(0);
});

test('addAllOrRemoveAllResources() adds all resources to pending request', async () => {
  const { accessRequestsService: service } = getTestSetup(
    getMockPendingResourceAccessRequest()
  );
  const server = makeServer({
    uri: `${rootClusterUri}/servers/ser`,
    hostname: 'ser',
  });
  const server2 = makeServer({
    uri: `${rootClusterUri}/servers/ser2`,
    hostname: 'ser2',
  });

  // add a single resource that isn't added should add to the request
  await service.addAllOrRemoveAllResources([
    { kind: 'server', resource: server },
  ]);
  let pendingAccessRequest = service.getPendingAccessRequest();
  expect(
    pendingAccessRequest.kind === 'resource' &&
      pendingAccessRequest.resources.get(server.uri)
  ).toStrictEqual({
    kind: 'server',
    resource: { hostname: server.hostname, uri: server.uri },
  });

  // padding an array that contains some resources already added and some that aren't should add them all
  await service.addAllOrRemoveAllResources([
    { kind: 'server', resource: server },
    { kind: 'server', resource: server2 },
  ]);
  pendingAccessRequest = service.getPendingAccessRequest();
  expect(
    pendingAccessRequest.kind === 'resource' &&
      pendingAccessRequest.resources.get(server.uri)
  ).toStrictEqual({
    kind: 'server',
    resource: { hostname: server.hostname, uri: server.uri },
  });
  expect(
    pendingAccessRequest.kind === 'resource' &&
      pendingAccessRequest.resources.get(server2.uri)
  ).toStrictEqual({
    kind: 'server',
    resource: { hostname: server2.hostname, uri: server2.uri },
  });

  // passing an array of resources that are all already added should remove all the passed resources
  await service.addAllOrRemoveAllResources([
    { kind: 'server', resource: server },
    { kind: 'server', resource: server2 },
  ]);
  pendingAccessRequest = service.getPendingAccessRequest();
  expect(
    pendingAccessRequest.kind === 'resource' &&
      pendingAccessRequest.resources.get(server.uri)
  ).toStrictEqual(undefined);
  expect(
    pendingAccessRequest.kind === 'resource' &&
      pendingAccessRequest.resources.get(server2.uri)
  ).toStrictEqual(undefined);
});

test('addOrRemoveResource() adds resource to pending request', async () => {
  const { accessRequestsService: service } = getTestSetup(
    getMockPendingResourceAccessRequest()
  );
  const server = makeServer({
    uri: `${rootClusterUri}/servers/ser2`,
    hostname: 'ser2',
  });
  await service.addOrRemoveResource({ kind: 'server', resource: server });
  const pendingAccessRequest = service.getPendingAccessRequest();
  expect(
    pendingAccessRequest.kind === 'resource' &&
      pendingAccessRequest.resources.get(server.uri)
  ).toStrictEqual({
    kind: 'server',
    resource: { hostname: server.hostname, uri: server.uri },
  });
});

test('addOrRemoveRole() adds role to pending request', async () => {
  const { accessRequestsService: service } = getTestSetup(
    getMockPendingRoleAccessRequest()
  );
  await service.addOrRemoveRole('requester');

  const pendingRequest = service.getPendingAccessRequest();
  expect(
    pendingRequest.kind === 'role' && Array.from(pendingRequest.roles.keys())
  ).toStrictEqual(['admin', 'requester']);
});

test('addOrRemoveResource() removes resource if it already exists in pending request', async () => {
  const { accessRequestsService: service } = getTestSetup(
    getMockPendingResourceAccessRequest()
  );
  const server = makeServer();
  await service.addOrRemoveResource({ kind: 'server', resource: server });
  const pendingAccessRequest = service.getPendingAccessRequest();
  expect(
    pendingAccessRequest.kind === 'resource' &&
      pendingAccessRequest.resources.has(server.uri)
  ).toBe(false);
});

test('addOrRemoveRole() removes role if it already exists in pending request', async () => {
  const { accessRequestsService: service } = getTestSetup(
    getMockPendingRoleAccessRequest()
  );
  await service.addOrRemoveRole('admin');

  const pendingRequest = service.getPendingAccessRequest();
  expect(
    pendingRequest.kind === 'role' && Array.from(pendingRequest.roles.keys())
  ).toStrictEqual([]);
});

test('resources from different clusters but with the same ID can be combined in a single request', async () => {
  const { accessRequestsService: service } = getTestSetup({
    kind: 'resource',
    resources: new Map(),
  });

  const server1 = makeServer({
    uri: `/clusters/${rootClusterUri}/servers/foo1`,
    hostname: 'foo1',
  });
  await service.addOrRemoveResource({
    kind: 'server',
    resource: server1,
  });

  const server2 = makeServer({
    uri: `/clusters/${leafClusterUri}/servers/foo2`,
    hostname: 'foo2',
  });
  await service.addOrRemoveResource({
    kind: 'server',
    resource: server2,
  });

  const pendingRequest = service.getPendingAccessRequest();
  expect(
    pendingRequest.kind === 'resource' &&
      Array.from(pendingRequest.resources.entries())
  ).toStrictEqual([
    [
      server1.uri,
      {
        kind: 'server',
        resource: {
          hostname: server1.hostname,
          uri: server1.uri,
        },
      },
    ],
    [
      server2.uri,
      {
        kind: 'server',
        resource: {
          hostname: server2.hostname,
          uri: server2.uri,
        },
      },
    ],
  ]);
});

test('does not update the request when the user tries to mix roles with resources and does not agree to clear the current request', async () => {
  const { accessRequestsService, modalsService } = getTestSetup(
    getMockPendingRoleAccessRequest()
  );

  // Cancel the modal immediately.
  jest.spyOn(modalsService, 'openRegularDialog').mockImplementation(dialog => {
    if (dialog.kind === 'change-access-request-kind') {
      dialog.onCancel();
    } else {
      throw new Error(`Got unexpected dialog ${dialog.kind}`);
    }

    return {
      closeDialog: () => {},
    };
  });

  const server1 = makeServer({
    uri: `/clusters/${rootClusterUri}/servers/foo1`,
    hostname: 'foo1',
  });
  await accessRequestsService.addOrRemoveResource({
    kind: 'server',
    resource: server1,
  });

  const pendingRequest = accessRequestsService.getPendingAccessRequest();
  expect(
    pendingRequest.kind === 'role' && Array.from(pendingRequest.roles.keys())
  ).toStrictEqual(['admin']);
});

test('updates the request when the user tries to mix roles with resources and agrees to clear the current request', async () => {
  const { accessRequestsService, modalsService } = getTestSetup(
    getMockPendingRoleAccessRequest()
  );

  // Cancel the modal immediately.
  jest.spyOn(modalsService, 'openRegularDialog').mockImplementation(dialog => {
    if (dialog.kind === 'change-access-request-kind') {
      dialog.onConfirm();
    } else {
      throw new Error(`Got unexpected dialog ${dialog.kind}`);
    }

    return {
      closeDialog: () => {},
    };
  });

  const server = makeServer({
    uri: `/clusters/${rootClusterUri}/servers/foo1`,
    hostname: 'foo1',
  });
  await accessRequestsService.addOrRemoveResource({
    kind: 'server',
    resource: server,
  });

  const pendingRequest = accessRequestsService.getPendingAccessRequest();
  expect(
    pendingRequest.kind === 'resource' &&
      pendingRequest.resources.get(server.uri)
  ).toStrictEqual({
    kind: 'server',
    resource: {
      hostname: server.hostname,
      uri: server.uri,
    },
  });
});

test('adding the same resource twice with addResource does not toggle it', async () => {
  const { accessRequestsService } = getTestSetup(
    getMockPendingResourceAccessRequest()
  );

  const server = makeServer({
    uri: `/clusters/${rootClusterUri}/servers/foo1`,
    hostname: 'foo1',
  });

  // Try to add the same resource twice.
  await accessRequestsService.addResource({
    kind: 'server',
    resource: server,
  });
  await accessRequestsService.addResource({
    kind: 'server',
    resource: server,
  });

  const pendingRequest = accessRequestsService.getPendingAccessRequest();
  expect(
    pendingRequest.kind === 'resource' &&
      pendingRequest.resources.get(server.uri)
  ).toStrictEqual({
    kind: 'server',
    resource: {
      hostname: server.hostname,
      uri: server.uri,
    },
  });
});
