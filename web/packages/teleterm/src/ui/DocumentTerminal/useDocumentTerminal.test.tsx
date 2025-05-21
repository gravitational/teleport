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

import { renderHook, waitFor } from '@testing-library/react';

import 'jest-canvas-mock';

import Logger, { NullService } from 'teleterm/logger';
import { PtyCommand, PtyProcessCreationStatus } from 'teleterm/services/pty';
import {
  makeLeafCluster,
  makeRootCluster,
} from 'teleterm/services/tshd/testHelpers';
import { IPtyProcess } from 'teleterm/sharedProcess/ptyHost';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import {
  DocumentPtySession,
  DocumentTerminal,
  DocumentTshNode,
} from 'teleterm/ui/services/workspacesService';
import type * as uri from 'teleterm/ui/uri';

import { WorkspaceContextProvider } from '../Documents';
import { useDocumentTerminal } from './useDocumentTerminal';

beforeAll(() => {
  Logger.init(new NullService());
});

beforeEach(() => {
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
  leafClusterId: undefined,
  login: 'user',
  origin: 'resource_table',
});

const getDocPtySession: () => DocumentPtySession = () => ({
  kind: 'doc.terminal_shell',
  title: 'Terminal',
  uri: '/docs/456',
  rootClusterId: 'test',
});

const getPtyProcessMock = (): IPtyProcess => ({
  onOpen: jest.fn(),
  write: jest.fn(),
  resize: jest.fn(),
  dispose: jest.fn(),
  onData: jest.fn(),
  start: jest.fn(),
  onStartError: jest.fn(),
  onExit: jest.fn(),
  getCwd: jest.fn(),
  getPtyId: jest.fn(),
});

test('useDocumentTerminal calls TerminalsService during init', async () => {
  const doc = getDocTshNode();
  const { wrapper, appContext } = testSetup(doc);

  const { result } = renderHook(() => useDocumentTerminal(doc), { wrapper });

  await waitFor(() => expect(result.current.attempt.status).toBe('success'));

  const expectedPtyCommand: PtyCommand = {
    kind: 'pty.tsh-login',
    proxyHost: 'localhost:3080',
    clusterName: 'Test',
    login: 'user',
    serverId: serverUUID,
    rootClusterId: 'test',
    leafClusterId: undefined,
  };

  expect(result.current.attempt.statusText).toBeFalsy();
  expect(result.current.attempt.status).toBe('success');
  expect(appContext.terminalsService.createPtyProcess).toHaveBeenCalledWith(
    expectedPtyCommand
  );
});

test('useDocumentTerminal calls TerminalsService only once', async () => {
  const doc = getDocTshNode();
  const { wrapper, appContext } = testSetup(doc);

  const { result, rerender } = renderHook(() => useDocumentTerminal(doc), {
    wrapper,
  });

  await waitFor(() => expect(result.current.attempt.status).toBe('success'));
  expect(result.current.attempt.statusText).toBeFalsy();
  expect(result.current.attempt.status).toBe('success');
  rerender();

  expect(appContext.terminalsService.createPtyProcess).toHaveBeenCalledTimes(1);
});

test('useDocumentTerminal gets leaf cluster ID from ClustersService when the leaf cluster is in ClustersService', async () => {
  const doc = getDocTshNode();
  doc.leafClusterId = 'leaf';
  doc.serverUri = `${leafClusterUri}/servers/${doc.serverId}`;
  const { wrapper, appContext } = testSetup(doc, leafClusterUri);

  const { result } = renderHook(() => useDocumentTerminal(doc), { wrapper });

  await waitFor(() => expect(result.current.attempt.status).toBe('success'));

  const expectedPtyCommand: PtyCommand = {
    kind: 'pty.tsh-login',
    proxyHost: 'localhost:3080',
    clusterName: 'leaf',
    login: 'user',
    serverId: serverUUID,
    rootClusterId: 'test',
    leafClusterId: 'leaf',
  };

  expect(result.current.attempt.statusText).toBeFalsy();
  expect(result.current.attempt.status).toBe('success');
  expect(appContext.terminalsService.createPtyProcess).toHaveBeenCalledWith(
    expectedPtyCommand
  );
});

test('useDocumentTerminal gets leaf cluster ID from doc.leafClusterId if the leaf cluster is not synced yet', async () => {
  const doc = getDocTshNode();
  doc.leafClusterId = 'leaf';
  doc.serverUri = `${leafClusterUri}/servers/${doc.serverId}`;
  const { wrapper, appContext } = testSetup(doc, leafClusterUri);
  appContext.clustersService.setState(draft => {
    draft.clusters.delete(leafClusterUri);
  });

  const { result } = renderHook(() => useDocumentTerminal(doc), { wrapper });

  await waitFor(() => expect(result.current.attempt.status).toBe('success'));

  const expectedPtyCommand: PtyCommand = {
    kind: 'pty.tsh-login',
    proxyHost: 'localhost:3080',
    clusterName: 'leaf',
    login: 'user',
    serverId: serverUUID,
    rootClusterId: 'test',
    leafClusterId: 'leaf',
  };

  expect(result.current.attempt.statusText).toBeFalsy();
  expect(result.current.attempt.status).toBe('success');
  expect(appContext.terminalsService.createPtyProcess).toHaveBeenCalledWith(
    expectedPtyCommand
  );
});

test('useDocumentTerminal returns a failed attempt if the call to TerminalsService fails', async () => {
  const doc = getDocTshNode();
  const { wrapper, appContext } = testSetup(doc);
  const { terminalsService } = appContext;

  (
    terminalsService.createPtyProcess as jest.MockedFunction<
      typeof terminalsService.createPtyProcess
    >
  ).mockReset();
  jest
    .spyOn(terminalsService, 'createPtyProcess')
    .mockRejectedValue(new Error('whoops'));

  const { result } = renderHook(() => useDocumentTerminal(doc), { wrapper });

  await waitFor(() => expect(result.current.attempt.status).toBe('error'));
  const { attempt } = result.current;
  expect(attempt.statusText).toBe('whoops');
  expect(attempt.status).toBe('error');
});

test('useDocumentTerminal shows a warning notification if the call to TerminalsService fails due to resolving env timeout', async () => {
  const doc = getDocTshNode();
  const { wrapper, appContext } = testSetup(doc);
  const { terminalsService, notificationsService } = appContext;

  (
    terminalsService.createPtyProcess as jest.MockedFunction<
      typeof terminalsService.createPtyProcess
    >
  ).mockReset();
  jest.spyOn(terminalsService, 'createPtyProcess').mockResolvedValue({
    process: getPtyProcessMock(),
    creationStatus: PtyProcessCreationStatus.ResolveShellEnvTimeout,
    windowsPty: undefined,
    shell: {
      id: 'zsh',
      friendlyName: 'zsh',
      binPath: '/bin/zsh',
      binName: 'zsh',
    },
  });
  jest.spyOn(notificationsService, 'notifyWarning');

  const { result } = renderHook(() => useDocumentTerminal(doc), { wrapper });

  await waitFor(() => expect(result.current.attempt.status).toBe('success'));
  expect(result.current.attempt.statusText).toBeFalsy();
  expect(result.current.attempt.status).toBe('success');

  expect(notificationsService.notifyWarning).toHaveBeenCalledWith({
    title: expect.stringContaining('Could not source environment variables'),
    description: expect.stringContaining('shell startup'),
  });
  expect(notificationsService.notifyWarning).toHaveBeenCalledTimes(1);
});

// testSetup adds a cluster to ClustersService and WorkspacesService.
// It also makes TerminalsService.prototype.createPtyProcess a noop.
const testSetup = (
  doc: DocumentTerminal,
  localClusterUri: uri.ClusterUri = rootClusterUri
) => {
  const cluster = makeRootCluster({
    uri: rootClusterUri,
    name: 'Test',
    proxyHost: 'localhost:3080',
  });
  const leafCluster = makeLeafCluster({
    uri: leafClusterUri,
    name: 'leaf',
  });
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
        process: getPtyProcessMock(),
        creationStatus: PtyProcessCreationStatus.Ok,
        windowsPty: undefined,
        shell: {
          id: 'zsh',
          friendlyName: 'zsh',
          binPath: '/bin/zsh',
          binName: 'zsh',
        },
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

  return { appContext, wrapper, documentsService };
};

test('shellId is set to a config default when empty', async () => {
  const doc = getDocPtySession();
  const { wrapper, appContext } = testSetup(doc);
  appContext.configService.set('terminal.shell', 'bash');
  const { terminalsService } = appContext;

  (
    terminalsService.createPtyProcess as jest.MockedFunction<
      typeof terminalsService.createPtyProcess
    >
  ).mockReset();
  jest.spyOn(terminalsService, 'createPtyProcess').mockResolvedValue({
    process: getPtyProcessMock(),
    creationStatus: PtyProcessCreationStatus.Ok,
    windowsPty: undefined,
    shell: {
      id: 'zsh',
      friendlyName: 'zsh',
      binPath: '/bin/zsh',
      binName: 'zsh',
    },
  });

  const { result } = renderHook(() => useDocumentTerminal(doc), { wrapper });

  await waitFor(() => expect(result.current.attempt.status).toBe('success'));
  expect(terminalsService.createPtyProcess).toHaveBeenCalledWith({
    shellId: 'bash',
    clusterName: 'Test',
    cwd: undefined,
    kind: 'pty.shell',
    proxyHost: 'localhost:3080',
    rootClusterId: 'test',
    title: 'Terminal',
    uri: '/docs/456',
  });
});

// TODO(ravicious): Add tests for the following cases:
// * dispose on unmount when state is success
// * removing init command from doc
// * marking the doc as connected when data arrives
// * closing the doc with 0 exit code
// * not closing the doc with non-zero exit code
