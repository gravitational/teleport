/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import userEvent from '@testing-library/user-event';

import { render, screen } from 'design/utils/testing';
import { Gateway } from 'gen-proto-ts/teleport/lib/teleterm/v1/gateway_pb';

import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import {
  makeAppGateway,
  makeRootCluster,
} from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import type * as docs from 'teleterm/ui/services/workspacesService';
import { AppUri } from 'teleterm/ui/uri';

import { DocumentGatewayApp } from './DocumentGatewayApp';

beforeEach(() => {
  jest.restoreAllMocks();
});

describe('reconnecting when the gateway fails to be created', () => {
  const tests: Array<{
    name: string;
    gateway: Gateway;
  }> = [
    {
      name: 'web app',
      gateway: makeAppGateway({
        protocol: 'HTTP',
        targetSubresourceName: undefined,
      }),
    },
    {
      name: 'single-port TCP app',
      gateway: makeAppGateway({
        protocol: 'TCP',
        targetSubresourceName: undefined,
      }),
    },
    {
      name: 'multi-port TCP app',
      gateway: makeAppGateway({
        protocol: 'TCP',
        targetSubresourceName: '1337',
      }),
    },
  ];

  test.each(tests)('$name', async ({ gateway }) => {
    const user = userEvent.setup();

    const appContext = new MockAppContext();
    const cluster = makeRootCluster();
    const doc: docs.DocumentGateway = {
      uri: '/docs/1',
      kind: 'doc.gateway',
      targetName: gateway.targetName,
      targetUri: gateway.targetUri as AppUri,
      targetUser: gateway.targetUser,
      targetSubresourceName: gateway.targetSubresourceName,
      gatewayUri: gateway.uri,
      origin: 'resource_table',
      title: '',
      status: '',
    };
    appContext.addRootClusterWithDoc(cluster, doc);

    jest
      .spyOn(appContext.tshd, 'createGateway')
      .mockReturnValueOnce(
        new MockedUnaryCall(undefined, new Error('Something went wrong'))
      )
      .mockReturnValueOnce(new MockedUnaryCall(gateway));

    render(
      <MockAppContextProvider appContext={appContext}>
        <MockWorkspaceContextProvider>
          <DocumentGatewayApp visible doc={doc} />
        </MockWorkspaceContextProvider>
      </MockAppContextProvider>
    );

    expect(
      await screen.findByText('Could not establish the connection')
    ).toBeInTheDocument();

    await user.click(screen.getByText('Reconnect'));

    expect(await screen.findByText('Close Connection')).toBeInTheDocument();
  });

  it('allows changing the target port for multi-port TCP apps', async () => {
    const user = userEvent.setup();

    const appContext = new MockAppContext();
    const cluster = makeRootCluster();
    const gateway = makeAppGateway({
      protocol: 'TCP',
      targetSubresourceName: '1337',
    });
    const doc: docs.DocumentGateway = {
      uri: '/docs/1',
      kind: 'doc.gateway',
      targetName: gateway.targetName,
      targetUri: gateway.targetUri as AppUri,
      targetUser: gateway.targetUser,
      targetSubresourceName: '1337',
      gatewayUri: gateway.uri,
      origin: 'resource_table',
      title: '',
      status: '',
    };
    appContext.addRootClusterWithDoc(cluster, doc);

    jest
      .spyOn(appContext.tshd, 'createGateway')
      .mockReturnValueOnce(
        new MockedUnaryCall(undefined, new Error('Something went wrong'))
      )
      .mockImplementationOnce(
        async req => new MockedUnaryCall({ ...gateway, ...req })
      );

    render(
      <MockAppContextProvider appContext={appContext}>
        <MockWorkspaceContextProvider>
          <DocumentGatewayApp visible doc={doc} />
        </MockWorkspaceContextProvider>
      </MockAppContextProvider>
    );

    expect(
      await screen.findByText('Could not establish the connection')
    ).toBeInTheDocument();

    const targetPortInput = screen.getByLabelText('Target Port *');
    await user.clear(targetPortInput);
    await user.type(targetPortInput, '4242');
    await user.click(screen.getByText('Reconnect'));

    expect(await screen.findByText('Close Connection')).toBeInTheDocument();
    expect(screen.getByLabelText('Target Port *')).toHaveValue(4242);

    expect(appContext.tshd.createGateway).toHaveBeenLastCalledWith(
      expect.objectContaining({
        targetSubresourceName: '4242',
      })
    );
  });
});
