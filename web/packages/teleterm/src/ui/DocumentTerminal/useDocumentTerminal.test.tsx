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
  makeServer,
} from 'teleterm/services/tshd/testHelpers';
import type * as tsh from 'teleterm/services/tshd/types';
import { IPtyProcess } from 'teleterm/sharedProcess/ptyHost';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import {
  AmbiguousHostnameError,
  ResourcesService,
} from 'teleterm/ui/services/resources';
import {
  DocumentPtySession,
  DocumentTerminal,
  DocumentTshNode,
  DocumentTshNodeWithLoginHost,
  DocumentTshNodeWithServerId,
} from 'teleterm/ui/services/workspacesService';
import type { IAppContext } from 'teleterm/ui/types';
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
const server: tsh.Server = makeServer({
  uri: `${rootClusterUri}/servers/${serverUUID}`,
  name: serverUUID,
});
const leafServer = makeServer({
  ...server,
  uri: `${leafClusterUri}/servers/${serverUUID}`,
});

const getDocTshNodeWithServerId: () => DocumentTshNodeWithServerId = () => ({
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

const getDocTshNodeWithLoginHost: () => DocumentTshNodeWithLoginHost = () => {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const { serverId, serverUri, login, ...rest } = getDocTshNodeWithServerId();
  return {
    ...rest,
    loginHost: 'user@foo',
  };
};

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
  const doc = getDocTshNodeWithServerId();
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
  const doc = getDocTshNodeWithServerId();
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
  const doc = getDocTshNodeWithServerId();
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
  const doc = getDocTshNodeWithServerId();
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
  const doc = getDocTshNodeWithServerId();
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
  const doc = getDocTshNodeWithServerId();
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

describe('calling useDocumentTerminal with a doc with a loginHost', () => {
  const tests: Array<
    {
      name: string;
      prepareDoc?: (doc: DocumentTshNodeWithLoginHost) => void;
      prepareContext?: (ctx: IAppContext) => void;
      mockGetServerByHostname:
        | Awaited<ReturnType<ResourcesService['getServerByHostname']>>
        | AmbiguousHostnameError
        | Error;
      expectedDocumentUpdate: Partial<DocumentTshNode>;
      expectedArgsOfGetServerByHostname: Parameters<
        ResourcesService['getServerByHostname']
      >;
    } & (
      | { expectedPtyCommand: PtyCommand; expectedError?: never }
      | { expectedPtyCommand?: never; expectedError: string }
    )
  > = [
    {
      name: 'calls ResourcesService to resolve the hostname of a root cluster SSH server to a UUID',
      mockGetServerByHostname: server,
      expectedPtyCommand: {
        kind: 'pty.tsh-login',
        proxyHost: 'localhost:3080',
        clusterName: 'Test',
        login: 'user',
        serverId: serverUUID,
        rootClusterId: 'test',
        leafClusterId: undefined,
      },
      expectedDocumentUpdate: {
        serverId: serverUUID,
        serverUri: server.uri,
        login: 'user',
        loginHost: undefined,
        title: 'user@foo',
      },
      expectedArgsOfGetServerByHostname: [rootClusterUri, 'foo'],
    },
    {
      name: 'calls ResourcesService to resolve the hostname of a leaf cluster SSH server to a UUID',
      prepareDoc: doc => {
        doc.leafClusterId = 'leaf';
      },
      mockGetServerByHostname: leafServer,
      expectedPtyCommand: {
        kind: 'pty.tsh-login',
        proxyHost: 'localhost:3080',
        clusterName: 'leaf',
        login: 'user',
        serverId: serverUUID,
        rootClusterId: 'test',
        leafClusterId: 'leaf',
      },
      expectedDocumentUpdate: {
        serverId: serverUUID,
        serverUri: leafServer.uri,
        login: 'user',
        loginHost: undefined,
        title: 'user@foo',
      },
      expectedArgsOfGetServerByHostname: [leafClusterUri, 'foo'],
    },
    {
      name: 'starts the session even if the leaf cluster is not synced yet',
      prepareDoc: doc => {
        doc.leafClusterId = 'leaf';
      },
      prepareContext: ctx => {
        ctx.clustersService.setState(draft => {
          draft.clusters.delete(leafClusterUri);
        });
      },
      mockGetServerByHostname: leafServer,
      expectedPtyCommand: {
        kind: 'pty.tsh-login',
        proxyHost: 'localhost:3080',
        clusterName: 'leaf',
        login: 'user',
        serverId: serverUUID,
        rootClusterId: 'test',
        leafClusterId: 'leaf',
      },
      expectedDocumentUpdate: {
        serverId: serverUUID,
        serverUri: leafServer.uri,
        login: 'user',
        loginHost: undefined,
        title: 'user@foo',
      },
      expectedArgsOfGetServerByHostname: [leafClusterUri, 'foo'],
    },
    {
      name: 'maintains incorrect loginHost with too many parts',
      prepareDoc: doc => {
        doc.loginHost = 'user@foo@baz';
      },
      mockGetServerByHostname: undefined,
      expectedPtyCommand: {
        kind: 'pty.tsh-login',
        proxyHost: 'localhost:3080',
        clusterName: 'Test',
        login: 'user@foo',
        serverId: 'baz',
        rootClusterId: 'test',
        leafClusterId: undefined,
      },
      expectedDocumentUpdate: {
        serverId: 'baz',
        serverUri: `${rootClusterUri}/servers/baz`,
        login: 'user@foo',
        loginHost: undefined,
        title: 'user@foo@baz',
      },
      expectedArgsOfGetServerByHostname: [rootClusterUri, 'baz'],
    },
    {
      // This is in order to call `tsh ssh user@foo` anyway and make tsh show an appropriate error.
      name: 'uses hostname as serverId if no matching server was found',
      mockGetServerByHostname: undefined,
      expectedPtyCommand: {
        kind: 'pty.tsh-login',
        proxyHost: 'localhost:3080',
        clusterName: 'Test',
        login: 'user',
        serverId: 'foo',
        rootClusterId: 'test',
        leafClusterId: undefined,
      },
      expectedDocumentUpdate: {
        serverId: 'foo',
        serverUri: `${rootClusterUri}/servers/foo`,
        login: 'user',
        loginHost: undefined,
        title: 'user@foo',
      },
      expectedArgsOfGetServerByHostname: [rootClusterUri, 'foo'],
    },
    {
      // This is the case when the user tries to execute `tsh ssh host`. We want to call `tsh ssh
      // host` anyway and make tsh show an appropriate error. But…
      name: 'attempts to connect even if only the host was supplied and the server was not resolved',
      prepareDoc: doc => {
        doc.loginHost = 'host';
      },
      mockGetServerByHostname: undefined,
      expectedPtyCommand: {
        kind: 'pty.tsh-login',
        proxyHost: 'localhost:3080',
        clusterName: 'Test',
        login: undefined,
        serverId: 'host',
        rootClusterId: 'test',
        leafClusterId: undefined,
      },
      expectedDocumentUpdate: {
        serverId: 'host',
        serverUri: `${rootClusterUri}/servers/host`,
        login: undefined,
        loginHost: undefined,
        title: 'host',
      },
      expectedArgsOfGetServerByHostname: [rootClusterUri, 'host'],
    },
    {
      // …it might also be the case that the username of a Teleport user is equal to a user on the
      // host, in which case explicitly providing the username is not necessary.
      name: 'attempts to connect even if only the host was supplied and the server was resolved',
      prepareDoc: doc => {
        doc.loginHost = 'foo';
      },
      mockGetServerByHostname: server,
      expectedPtyCommand: {
        kind: 'pty.tsh-login',
        proxyHost: 'localhost:3080',
        clusterName: 'Test',
        login: undefined,
        serverId: serverUUID,
        rootClusterId: 'test',
        leafClusterId: undefined,
      },
      expectedDocumentUpdate: {
        serverId: serverUUID,
        serverUri: server.uri,
        login: undefined,
        loginHost: undefined,
        title: 'foo',
      },
      expectedArgsOfGetServerByHostname: [rootClusterUri, 'foo'],
    },
    {
      // As in other scenarios, we execute `tsh ssh user@ambiguous-host` anyway and let tsh show the
      // error message.
      name: 'silently ignores an ambiguous hostname error',
      prepareDoc: doc => {
        doc.loginHost = 'user@ambiguous-host';
      },
      mockGetServerByHostname: new AmbiguousHostnameError('ambiguous-host'),
      expectedPtyCommand: {
        kind: 'pty.tsh-login',
        proxyHost: 'localhost:3080',
        clusterName: 'Test',
        login: 'user',
        serverId: 'ambiguous-host',
        rootClusterId: 'test',
        leafClusterId: undefined,
      },
      expectedDocumentUpdate: {
        serverId: 'ambiguous-host',
        serverUri: `${rootClusterUri}/servers/ambiguous-host`,
        login: 'user',
        loginHost: undefined,
        title: 'user@ambiguous-host',
      },
      expectedArgsOfGetServerByHostname: [rootClusterUri, 'ambiguous-host'],
    },
    {
      name: 'returns a failed attempt if there was an error when resolving hostname',
      mockGetServerByHostname: new Error('oops'),
      expectedError: 'oops',
      expectedDocumentUpdate: { status: 'error' },
      expectedArgsOfGetServerByHostname: [rootClusterUri, 'foo'],
    },
  ];

  test.each(tests)(
    '$name',
    async ({
      prepareDoc,
      prepareContext,
      mockGetServerByHostname,
      expectedPtyCommand,
      expectedDocumentUpdate,
      expectedArgsOfGetServerByHostname,
      expectedError,
    }) => {
      const doc = getDocTshNodeWithLoginHost();
      prepareDoc?.(doc);
      const { wrapper, appContext, documentsService } = testSetup(doc);
      prepareContext?.(appContext);
      const { resourcesService, terminalsService } = appContext;

      jest.spyOn(documentsService, 'update');

      if (mockGetServerByHostname instanceof Error) {
        jest
          .spyOn(resourcesService, 'getServerByHostname')
          .mockRejectedValueOnce(mockGetServerByHostname);
      } else {
        jest
          .spyOn(resourcesService, 'getServerByHostname')
          .mockResolvedValueOnce(mockGetServerByHostname);
      }

      const { result } = renderHook(() => useDocumentTerminal(doc), {
        wrapper,
      });

      await waitFor(() =>
        expect(result.current.attempt.status).toBe(
          expectedError ? 'error' : 'success'
        )
      );

      const { attempt } = result.current;
      /* eslint-disable jest/no-conditional-expect */
      if (expectedError) {
        expect(attempt.statusText).toBe(expectedError);
        expect(attempt.status).toBe('error');
        expect(terminalsService.createPtyProcess).not.toHaveBeenCalled();
      } else {
        expect(attempt.statusText).toBeFalsy();
        expect(attempt.status).toBe('success');
        expect(terminalsService.createPtyProcess).toHaveBeenCalledWith(
          expectedPtyCommand
        );
      }
      /* eslint-enable jest/no-conditional-expect */

      expect(resourcesService.getServerByHostname).toHaveBeenCalledWith(
        ...expectedArgsOfGetServerByHostname
      );
      expect(documentsService.update).toHaveBeenCalledWith(
        doc.uri,
        expectedDocumentUpdate
      );
    }
  );
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
