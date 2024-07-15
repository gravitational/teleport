/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import React from 'react';

import { wait } from 'shared/utils/wait';

import { DocumentGatewayApp } from 'teleterm/ui/DocumentGatewayApp/DocumentGatewayApp';
import * as types from 'teleterm/ui/services/workspacesService';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import { makeAppGateway } from 'teleterm/services/tshd/testHelpers';
import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';

export default {
  title: 'Teleterm/DocumentGatewayApp',
};

const gateway = makeAppGateway();

const documentGateway: types.DocumentGateway = {
  kind: 'doc.gateway',
  targetUri: '/clusters/bar/apps/quux',
  origin: 'resource_table',
  gatewayUri: gateway.uri,
  uri: '/docs/123',
  title: 'quux',
  targetUser: '',
  status: '',
  targetName: 'quux',
};

const rootClusterUri = '/clusters/bar';

export function Online() {
  const appContext = new MockAppContext();
  appContext.clustersService.createGateway = () => Promise.resolve(gateway);
  appContext.clustersService.setState(draftState => {
    draftState.gateways.set(gateway.uri, gateway);
  });

  appContext.workspacesService.setState(draftState => {
    draftState.rootClusterUri = rootClusterUri;
    draftState.workspaces[rootClusterUri] = {
      localClusterUri: rootClusterUri,
      documents: [documentGateway],
      location: documentGateway.uri,
      accessRequests: undefined,
    };
  });

  appContext.tshd.setGatewayLocalPort = ({ localPort }) =>
    wait(800).then(() => new MockedUnaryCall({ ...gateway, localPort }));

  return (
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider>
        <DocumentGatewayApp doc={documentGateway} visible={true} />
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );
}

export function Offline() {
  const offlineDocumentGateway = { ...documentGateway, gatewayUri: undefined };
  const appContext = new MockAppContext();
  appContext.clustersService.createGateway = () =>
    Promise.reject(new Error('failed to create gateway'));

  appContext.workspacesService.setState(draftState => {
    draftState.rootClusterUri = rootClusterUri;
    draftState.workspaces[rootClusterUri] = {
      localClusterUri: rootClusterUri,
      documents: [offlineDocumentGateway],
      location: offlineDocumentGateway.uri,
      accessRequests: undefined,
    };
  });

  return (
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider>
        <DocumentGatewayApp doc={offlineDocumentGateway} visible={true} />
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );
}
