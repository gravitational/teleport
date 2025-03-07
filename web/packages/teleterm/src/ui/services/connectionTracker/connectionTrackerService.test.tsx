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

import 'jest-canvas-mock';

import { within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { act, ComponentType, createRef } from 'react';

import { render, screen } from 'design/utils/testing';
import { App } from 'gen-proto-ts/teleport/lib/teleterm/v1/app_pb';

import Logger, { NullService } from 'teleterm/logger';
import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import {
  makeApp,
  makeAppGateway,
  makeRootCluster,
} from 'teleterm/services/tshd/testHelpers';
import { ResourcesContextProvider } from 'teleterm/ui/DocumentCluster/resourcesContext';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { DocumentsService } from 'teleterm/ui/services/workspacesService';
import { TabHost } from 'teleterm/ui/TabHost';
import { IAppContext } from 'teleterm/ui/types';
import { unique } from 'teleterm/ui/utils';

import { TrackedGatewayConnection } from './types';

beforeAll(() => {
  Logger.init(new NullService());
});

test('updating target port creates new connection', async () => {
  const user = userEvent.setup();
  const { ctx, docsService, app, Component } = setupTests();

  const doc1 = docsService.createGatewayDocument({
    targetName: app.name,
    targetUri: app.uri,
    targetUser: undefined,
    targetSubresourceName: '1337',
    origin: 'resource_table',
  });
  // Add without opening. It's not necessary and it'll be easier to verify activating connections
  // later if we don't open the doc at this point.
  docsService.add(doc1);

  render(<Component />);

  // Wait for the gateway to be created.
  expect(await screen.findByText('Close Connection')).toBeInTheDocument();
  expect(ctx.connectionTracker.getConnections()).toHaveLength(1);
  const conn1337 = ctx.connectionTracker.getConnections()[0];
  expect(conn1337.title).toEqual(`${app.name}:1337`);

  // Update target port.
  let targetPortInput = screen.getByLabelText('Target Port *');
  await user.clear(targetPortInput);
  await user.type(targetPortInput, '4242');
  // We have to lose focus of that field, otherwise React is going to warn about updates not wrapped
  // in act when the focus on the page changes after opening a new doc.
  await user.tab();
  expect(
    await screen.findByTitle('Target Port successfully updated', undefined, {
      // There's a 1s debounce on port fields.
      timeout: 2000,
    })
  ).toBeInTheDocument();

  // Verify connections.
  expect(ctx.connectionTracker.getConnections()).toHaveLength(2);
  const conn4242 = ctx.connectionTracker.getConnections()[1];
  expect(conn4242.id).not.toEqual(conn1337.id);
  expect(conn4242.title).toEqual(`${app.name}:4242`);

  await act(async () => {
    await ctx.connectionTracker.activateItem(conn4242.id, {
      origin: 'resource_table',
    });
  });
  expect(docsService.getLocation()).toEqual(doc1.uri);

  await act(async () => {
    await ctx.connectionTracker.activateItem(conn1337.id, {
      origin: 'resource_table',
    });
  });
  expect(docsService.getDocuments()).toHaveLength(2);
  expect(docsService.getLocation()).not.toEqual(doc1.uri);
});

test('updating target port to match connection params of gateway created by other doc is possible', async () => {
  const user = userEvent.setup();
  const { ctx, docsService, app, Component } = setupTests();

  const baseDocumentGatewayFields = {
    targetName: app.name,
    targetUri: app.uri,
    targetUser: undefined,
    origin: 'resource_table' as const,
  };
  const doc1 = docsService.createGatewayDocument({
    ...baseDocumentGatewayFields,
    targetSubresourceName: '1337',
  });
  // Add without opening. It's not necessary and it'll be easier to verify activating connections
  // later if we don't open the doc at this point.
  docsService.add(doc1);

  render(<Component />);

  // Wait for the gateways to be created.
  expect(await screen.findByText('Close Connection')).toBeInTheDocument();
  expect(ctx.connectionTracker.getConnections()).toHaveLength(1);
  const conn1337Id = ctx.connectionTracker.getConnections()[0].id;

  // Create a second gateway.
  const doc2 = docsService.createGatewayDocument({
    ...baseDocumentGatewayFields,
    targetSubresourceName: '4242',
  });
  await act(async () => {
    docsService.add(doc2);
  });
  const doc2Node = await screen.findByTestId(doc2.uri);
  expect(
    await within(doc2Node).findByText('Close Connection')
  ).toBeInTheDocument();
  expect(ctx.connectionTracker.getConnections()).toHaveLength(2);
  const conn4242Id = ctx.connectionTracker.getConnections()[1].id;
  expect(conn4242Id).not.toEqual(conn1337Id);

  // Close the second gateway.
  await user.click(within(doc2Node).getByText('Close Connection'));
  expect(ctx.connectionTracker.findConnection(conn4242Id).connected).toBe(
    false
  );
  expect(ctx.connectionTracker.findConnection(conn1337Id).connected).toBe(true);

  // Update target port from 1337 to 4242.
  let targetPortInput = screen.getByLabelText('Target Port *');
  await user.clear(targetPortInput);
  await user.type(targetPortInput, '4242');
  await user.tab();
  expect(
    await screen.findByTitle('Target Port successfully updated', undefined, {
      // There's a 1s debounce on port fields.
      timeout: 2000,
    })
  ).toBeInTheDocument();

  // Verify that connection for 4242 is now connected and the connection for 1337 went offline.
  expect(ctx.connectionTracker.getConnections()).toHaveLength(2);
  const conn4242 = ctx.connectionTracker.findConnection(
    conn4242Id
  ) as TrackedGatewayConnection;
  const conn1337 = ctx.connectionTracker.findConnection(
    conn1337Id
  ) as TrackedGatewayConnection;
  expect(conn4242.connected).toBe(true);
  expect(conn1337.connected).toBe(false);
  // The ports are expected to be the same. We just changed doc with port 1337 to port 4242, so the
  // corresponding connection has changed from conn1337 to conn4242. conn4242 got updated with the
  // port set on doc1.
  expect(conn4242).toBeTruthy();
  expect(conn4242.port).toEqual(conn1337.port);

  await act(async () => {
    await ctx.connectionTracker.activateItem(conn4242Id, {
      origin: 'resource_table',
    });
  });
  expect(docsService.getLocation()).toEqual(doc1.uri);
});

function setupTests(): {
  ctx: IAppContext;
  docsService: DocumentsService;
  app: App;
  Component: ComponentType;
} {
  const ctx = new MockAppContext();
  const rootCluster = makeRootCluster();
  ctx.addRootCluster(rootCluster);
  ctx.workspacesService.setState(draft => {
    draft.rootClusterUri = rootCluster.uri;
  });

  const docsService = ctx.workspacesService.getWorkspaceDocumentService(
    rootCluster.uri
  );

  const app = makeApp({
    tcpPorts: [
      { port: 1337, endPort: 0 },
      { port: 4242, endPort: 0 },
    ],
    endpointUri: 'tcp://localhost',
  });

  let gatewayLocalPort = 0;
  jest.spyOn(ctx.tshd, 'createGateway').mockImplementation(async req => {
    gatewayLocalPort++;

    return new MockedUnaryCall(
      makeAppGateway({
        ...req,
        protocol: 'TCP',
        uri: `/gateways/${unique()}`,
        localPort: req.localPort || gatewayLocalPort.toString(),
      })
    );
  });
  jest
    .spyOn(ctx.tshd, 'setGatewayTargetSubresourceName')
    .mockImplementation(async req => {
      const gateway = ctx.clustersService.findGateway(req.gatewayUri);
      const updatedGateway = {
        ...gateway,
        targetSubresourceName: req.targetSubresourceName,
      };

      return new MockedUnaryCall(updatedGateway);
    });
  jest
    .spyOn(ctx.tshd, 'getApp')
    .mockResolvedValue(new MockedUnaryCall({ app }));

  const topBarConnectMyComputerRef = createRef<HTMLDivElement>();
  const topBarAccessRequestRef = createRef<HTMLDivElement>();
  const Component = () => (
    <MockAppContextProvider appContext={ctx}>
      <ResourcesContextProvider>
        <TabHost
          ctx={ctx}
          topBarConnectMyComputerRef={topBarConnectMyComputerRef}
          topBarAccessRequestRef={topBarAccessRequestRef}
        />
      </ResourcesContextProvider>
    </MockAppContextProvider>
  );

  return { ctx, docsService, app, Component };
}
