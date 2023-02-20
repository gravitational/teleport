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
import { renderHook } from '@testing-library/react-hooks';
import 'jest-canvas-mock';
import * as useAsync from 'shared/hooks/useAsync';

import Logger, { NullService } from 'teleterm/logger';
import { PtyCommand, PtyProcessCreationStatus } from 'teleterm/services/pty';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { DocumentTshNode } from 'teleterm/ui/services/workspacesService';

import { WorkspaceContextProvider } from '../Documents';

import useDocumentTerminal from './useDocumentTerminal';

import type * as tsh from 'teleterm/services/tshd/types';
import type * as uri from 'teleterm/ui/uri';

beforeAll(() => {
  Logger.init(new NullService());
});

afterEach(() => {
  jest.restoreAllMocks();
});

const rootClusterUri = '/clusters/test' as const;
const leafClusterUri = `${rootClusterUri}/leaves/leaf` as const;
const serverUUID = 'bed30649-3af5-40f1-a832-54ff4adcca41';

const getDocTshNode: () => DocumentTshNode = () => ({
  kind: 'doc.terminal_tsh_node',
  uri: '/docs/123',
  title: '',
  status: '',
  serverId: serverUUID,
  serverUri: `${rootClusterUri}/servers/${serverUUID}`,
  rootClusterId: 'test',
  login: 'user',
});

test('useDocumentTerminal calls TerminalsService during init', async () => {
  const doc = getDocTshNode();
  const { wrapper, appContext } = testSetup();

  const { result, waitForValueToChange } = renderHook(
    () => useDocumentTerminal(doc),
    { wrapper }
  );

  await waitForValueToChange(() => useAsync.hasFinished(result.current));

  const expectedPtyCommand: PtyCommand = {
    kind: 'pty.tsh-login',
    proxyHost: 'localhost:3080',
    clusterName: 'Test',
    login: 'user',
    serverId: serverUUID,
    rootClusterId: 'test',
  };

  expect(result.current.statusText).toBeFalsy();
  expect(result.current.status).toBe('success');
  expect(appContext.terminalsService.createPtyProcess).toHaveBeenCalledWith(
    expect.objectContaining(expectedPtyCommand)
  );
});

test('useDocumentTerminal calls TerminalsService only once', async () => {
  const doc = getDocTshNode();
  const { wrapper, appContext } = testSetup();

  const { result, waitForValueToChange, rerender } = renderHook(
    () => useDocumentTerminal(doc),
    { wrapper }
  );

  await waitForValueToChange(() => useAsync.hasFinished(result.current));
  expect(result.current.statusText).toBeFalsy();
  expect(result.current.status).toBe('success');
  rerender();

  expect(appContext.terminalsService.createPtyProcess).toHaveBeenCalledTimes(1);
});

test('useDocumentTerminal gets leaf cluster ID from ClustersService when the leaf cluster is in ClustersService', async () => {
  const doc = getDocTshNode();
  doc.leafClusterId = 'leaf';
  doc.serverUri = `${leafClusterUri}/servers/${doc.serverId}`;
  const { wrapper, appContext } = testSetup(leafClusterUri);

  const { result, waitForValueToChange } = renderHook(
    () => useDocumentTerminal(doc),
    { wrapper }
  );

  await waitForValueToChange(() => useAsync.hasFinished(result.current));

  const expectedPtyCommand: PtyCommand = {
    kind: 'pty.tsh-login',
    proxyHost: 'localhost:3080',
    clusterName: 'leaf',
    login: 'user',
    serverId: serverUUID,
    rootClusterId: 'test',
    leafClusterId: 'leaf',
  };

  expect(result.current.statusText).toBeFalsy();
  expect(result.current.status).toBe('success');
  expect(appContext.terminalsService.createPtyProcess).toHaveBeenCalledWith(
    expect.objectContaining(expectedPtyCommand)
  );
});

test('useDocumentTerminal gets leaf cluster ID from doc.leafClusterId if the leaf cluster is not synced yet', async () => {
  const doc = getDocTshNode();
  doc.leafClusterId = 'leaf';
  doc.serverUri = `${leafClusterUri}/servers/${doc.serverId}`;
  const { wrapper, appContext } = testSetup(leafClusterUri);
  appContext.clustersService.setState(draft => {
    draft.clusters.delete(leafClusterUri);
  });

  const { result, waitForValueToChange } = renderHook(
    () => useDocumentTerminal(doc),
    { wrapper }
  );

  await waitForValueToChange(() => useAsync.hasFinished(result.current));

  const expectedPtyCommand: PtyCommand = {
    kind: 'pty.tsh-login',
    proxyHost: 'localhost:3080',
    clusterName: 'leaf',
    login: 'user',
    serverId: serverUUID,
    rootClusterId: 'test',
    leafClusterId: 'leaf',
  };

  expect(result.current.statusText).toBeFalsy();
  expect(result.current.status).toBe('success');
  expect(appContext.terminalsService.createPtyProcess).toHaveBeenCalledWith(
    expect.objectContaining(expectedPtyCommand)
  );
});

test('useDocumentTerminal shows an error notification if the call to TerminalsService fails', async () => {
  const doc = getDocTshNode();
  const { wrapper, appContext } = testSetup();
  const { terminalsService, notificationsService } = appContext;

  (
    terminalsService.createPtyProcess as jest.MockedFunction<
      typeof terminalsService.createPtyProcess
    >
  ).mockReset();
  jest
    .spyOn(terminalsService, 'createPtyProcess')
    .mockRejectedValue(new Error('whoops'));
  jest.spyOn(notificationsService, 'notifyError');

  const { result, waitForValueToChange } = renderHook(
    () => useDocumentTerminal(doc),
    { wrapper }
  );

  await waitForValueToChange(() => useAsync.hasFinished(result.current));
  expect(result.current.statusText).toBeFalsy();
  expect(result.current.status).toBe('success');

  expect(notificationsService.notifyError).toHaveBeenCalledWith('whoops');
  expect(notificationsService.notifyError).toHaveBeenCalledTimes(1);
});

test('useDocumentTerminal shows a warning notification if the call to TerminalsService fails due to resolving env timeout', async () => {
  const doc = getDocTshNode();
  const { wrapper, appContext } = testSetup();
  const { terminalsService, notificationsService } = appContext;

  (
    terminalsService.createPtyProcess as jest.MockedFunction<
      typeof terminalsService.createPtyProcess
    >
  ).mockReset();
  jest.spyOn(terminalsService, 'createPtyProcess').mockResolvedValue({
    process: undefined,
    creationStatus: PtyProcessCreationStatus.ResolveShellEnvTimeout,
  });
  jest.spyOn(notificationsService, 'notifyWarning');

  const { result, waitForValueToChange } = renderHook(
    () => useDocumentTerminal(doc),
    { wrapper }
  );

  await waitForValueToChange(() => useAsync.hasFinished(result.current));
  expect(result.current.statusText).toBeFalsy();
  expect(result.current.status).toBe('success');

  expect(notificationsService.notifyWarning).toHaveBeenCalledWith({
    title: expect.stringContaining('Could not source environment variables'),
    description: expect.stringContaining('shell startup'),
  });
  expect(notificationsService.notifyWarning).toHaveBeenCalledTimes(1);
});

// testSetup adds a cluster to ClustersService and WorkspacesService.
// It also makes TerminalsService.prototype.createPtyProcess a noop.
const testSetup = (localClusterUri: uri.ClusterUri = clusterUri) => {
  const cluster: tsh.Cluster = {
    uri: rootClusterUri,
    name: 'Test',
    connected: true,
    leaf: false,
    proxyHost: 'localhost:3080',
    authClusterId: '73c4746b-d956-4f16-9848-4e3469f70762',
    loggedInUser: {
      activeRequestsList: [],
      assumedRequests: {},
      name: 'admin',
      acl: {},
      sshLoginsList: [],
      rolesList: [],
      requestableRolesList: [],
      suggestedReviewersList: [],
    },
  };
  const leafCluster: tsh.Cluster = {
    uri: leafClusterUri,
    name: 'leaf',
    connected: true,
    leaf: true,
    proxyHost: '',
    authClusterId: '5408fc2f-a452-4bde-bda2-b3b918c635ad',
    loggedInUser: {
      activeRequestsList: [],
      assumedRequests: {},
      name: 'admin',
      acl: {},
      sshLoginsList: [],
      rolesList: [],
      requestableRolesList: [],
      suggestedReviewersList: [],
    },
  };
  const appContext = new MockAppContext();
  appContext.clustersService.setState(draftState => {
    draftState.clusters.set(rootClusterUri, cluster);
    draftState.clusters.set(leafCluster.uri, leafCluster);
  });
  appContext.workspacesService.setActiveWorkspace(rootClusterUri);
  const documentsService =
    appContext.workspacesService.getWorkspaceDocumentService(rootClusterUri);
  documentsService.add(doc);
  jest
    .spyOn(appContext.terminalsService, 'createPtyProcess')
    .mockImplementationOnce(async () => {
      return {
        process: undefined,
        creationStatus: PtyProcessCreationStatus.Ok,
      };
    });

  const wrapper = ({ children }) => (
    <MockAppContextProvider appContext={appContext}>
      <WorkspaceContextProvider
        value={{
          rootClusterUri: rootClusterUri,
          localClusterUri,
          documentsService,
          accessRequestsService: undefined,
        }}
      >
        {children}
      </WorkspaceContextProvider>
    </MockAppContextProvider>
  );

  return { appContext, wrapper };
};

// TODO(ravicious): Add tests for the following cases:
// * dispose on unmount when state is success
// * removing init command from doc
// * marking the doc as connected when data arrives
// * closing the doc with 0 exit code
// * not closing the doc with non-zero exit code
