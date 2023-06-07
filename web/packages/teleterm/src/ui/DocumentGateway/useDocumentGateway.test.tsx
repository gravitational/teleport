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

import React from 'react';
import { renderHook, act } from '@testing-library/react-hooks';

import {
  makeRootCluster,
  makeGateway,
} from 'teleterm/services/tshd/testHelpers';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { DocumentGateway } from 'teleterm/ui/services/workspacesService';

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

  const { result, waitFor } = renderHook(() => useDocumentGateway(doc), {
    wrapper: $wrapper,
  });

  await waitFor(() => result.current.connectAttempt.status === 'success');

  expect(appContext.clustersService.createGateway).toHaveBeenCalledWith({
    targetUri: doc.targetUri,
    subresource_name: doc.targetSubresourceName,
    user: doc.targetUser,
    port: doc.port,
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

  const { result, waitFor } = renderHook(() => useDocumentGateway(doc), {
    wrapper: $wrapper,
  });

  act(() => {
    result.current.disconnect();
  });

  await waitFor(() => result.current.disconnectAttempt.status === 'success');

  expect(appContext.clustersService.createGateway).not.toHaveBeenCalled();
});

const testSetup = () => {
  const appContext = new MockAppContext();
  const cluster = makeRootCluster({ connected: true });
  const gateway = makeGateway();
  const doc: DocumentGateway = {
    uri: '/docs/1',
    kind: 'doc.gateway',
    targetName: gateway.targetName,
    targetUri: gateway.targetUri,
    targetUser: gateway.targetUser,
    targetSubresourceName: gateway.targetSubresourceName,
    gatewayUri: gateway.uri,
    origin: 'resource_table',
    title: '',
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
