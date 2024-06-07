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

import React from 'react';
import { renderHook, act, waitFor } from '@testing-library/react';

import {
  makeRootCluster,
  makeDatabaseGateway,
} from 'teleterm/services/tshd/testHelpers';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { DocumentGateway } from 'teleterm/ui/services/workspacesService';
import { DatabaseUri } from 'teleterm/ui/uri';

import { WorkspaceContextProvider } from '../Documents';
import { MockAppContextProvider } from '../fixtures/MockAppContextProvider';

import { useDocumentGateway } from './useDocumentGateway';

beforeEach(() => {
  jest.restoreAllMocks();
});

it('creates a gateway on mount if it does not exist already', async () => {
  const { appContext, gateway, doc, $wrapper } = testSetup();

  jest
    .spyOn(appContext.clustersService, 'createGateway')
    .mockImplementation(async () => {
      appContext.clustersService.setState(draftState => {
        draftState.gateways.set(gateway.uri, gateway);
      });
      return gateway;
    });

  const { result } = renderHook(() => useDocumentGateway(doc), {
    wrapper: $wrapper,
  });

  await waitFor(() =>
    expect(result.current.connectAttempt.status).toBe('success')
  );

  expect(appContext.clustersService.createGateway).toHaveBeenCalledWith({
    targetUri: doc.targetUri,
    targetSubresourceName: doc.targetSubresourceName,
    targetUser: doc.targetUser,
    localPort: doc.port,
  });
  expect(appContext.clustersService.createGateway).toHaveBeenCalledTimes(1);
});

it('does not create a gateway on mount if the gateway already exists', async () => {
  const { appContext, gateway, doc, $wrapper } = testSetup();
  appContext.clustersService.setState(draftState => {
    draftState.gateways.set(gateway.uri, gateway);
  });
  jest.spyOn(appContext.clustersService, 'createGateway');

  renderHook(() => useDocumentGateway(doc), {
    wrapper: $wrapper,
  });

  expect(appContext.clustersService.createGateway).not.toHaveBeenCalled();
});

// Regression test.
it('does not attempt to create a gateway immediately after closing it if the gateway was already running', async () => {
  const { appContext, gateway, doc, $wrapper } = testSetup();
  appContext.clustersService.setState(draftState => {
    draftState.gateways.set(gateway.uri, gateway);
  });

  jest
    .spyOn(appContext.clustersService, 'removeGateway')
    .mockImplementation(async gatewayUri => {
      appContext.clustersService.setState(draftState => {
        draftState.gateways.delete(gatewayUri);
      });
    });
  jest
    .spyOn(appContext.clustersService, 'createGateway')
    .mockResolvedValue(gateway);

  const { result } = renderHook(() => useDocumentGateway(doc), {
    wrapper: $wrapper,
  });

  act(() => {
    result.current.disconnect();
  });

  await waitFor(() =>
    expect(result.current.disconnectAttempt.status).toBe('success')
  );

  expect(appContext.clustersService.createGateway).not.toHaveBeenCalled();
});

const testSetup = () => {
  const appContext = new MockAppContext();
  const cluster = makeRootCluster({ connected: true });
  const gateway = makeDatabaseGateway();
  const doc: DocumentGateway = {
    uri: '/docs/1',
    kind: 'doc.gateway',
    targetName: gateway.targetName,
    targetUri: gateway.targetUri as DatabaseUri,
    targetUser: gateway.targetUser,
    targetSubresourceName: gateway.targetSubresourceName,
    gatewayUri: gateway.uri,
    origin: 'resource_table',
    title: '',
    status: '',
  };
  appContext.clustersService.setState(draftState => {
    draftState.clusters.set(cluster.uri, cluster);
  });
  appContext.workspacesService.setState(draftState => {
    draftState.rootClusterUri = cluster.uri;
    draftState.workspaces[cluster.uri] = {
      documents: [doc],
      location: doc.uri,
      localClusterUri: cluster.uri,
      accessRequests: undefined,
    };
  });
  const workspaceContext = {
    rootClusterUri: cluster.uri,
    localClusterUri: cluster.uri,
    documentsService: appContext.workspacesService.getWorkspaceDocumentService(
      cluster.uri
    ),
    accessRequestsService: undefined,
  };
  const $wrapper = ({ children }) => (
    <MockAppContextProvider appContext={appContext}>
      <WorkspaceContextProvider value={workspaceContext}>
        {children}
      </WorkspaceContextProvider>
    </MockAppContextProvider>
  );

  return { gateway, doc, appContext, workspaceContext, $wrapper };
};
