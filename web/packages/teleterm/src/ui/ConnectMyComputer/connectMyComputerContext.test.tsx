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

import { EventEmitter } from 'node:events';

import React from 'react';
import { act, renderHook } from '@testing-library/react-hooks';
import { makeErrorAttempt } from 'shared/hooks/useAsync';

import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { WorkspaceContextProvider } from 'teleterm/ui/Documents';
import { AgentProcessState } from 'teleterm/mainProcess/types';

import {
  makeLoggedInUser,
  makeRootCluster,
  makeServer,
} from 'teleterm/services/tshd/testHelpers';
import { createMockConfigService } from 'teleterm/services/config/fixtures/mocks';

import {
  AgentProcessError,
  ConnectMyComputerContextProvider,
  useConnectMyComputerContext,
} from './connectMyComputerContext';

function getMocksWithConnectMyComputerEnabled() {
  const rootCluster = makeRootCluster({
    loggedInUser: makeLoggedInUser({
      acl: {
        tokens: {
          create: true,
          edit: false,
          list: false,
          use: false,
          read: false,
          pb_delete: false,
        },
      },
    }),
  });
  const appContext = new MockAppContext({});

  appContext.clustersService.setState(draftState => {
    draftState.clusters.set(rootCluster.uri, rootCluster);
  });
  appContext.workspacesService.setState(draftState => {
    draftState.rootClusterUri = rootCluster.uri;
    draftState.workspaces[rootCluster.uri] = {
      documents: [],
      location: undefined,
      localClusterUri: rootCluster.uri,
      accessRequests: undefined,
    };
  });
  appContext.configService = createMockConfigService({
    'feature.connectMyComputer': true,
  });

  return { appContext, rootCluster };
}

test('startAgent re-throws errors that are thrown while spawning the process', async () => {
  const { appContext, rootCluster } = getMocksWithConnectMyComputerEnabled();
  const eventEmitter = new EventEmitter();
  const errorStatus: AgentProcessState = {
    status: 'error',
    message: 'ENOENT',
  };

  jest
    .spyOn(appContext.connectMyComputerService, 'waitForNodeToJoin')
    .mockImplementation(
      // Hang until abort.
      (rootClusterUri, abortSignal) =>
        new Promise((resolve, reject) => abortSignal.addEventListener(reject))
    );
  jest
    .spyOn(appContext.mainProcessClient, 'getAgentState')
    .mockImplementation(() => errorStatus);
  jest
    .spyOn(appContext.connectMyComputerService, 'runAgent')
    .mockImplementation(async () => {
      // the error is emitted before the function resolves
      eventEmitter.emit('', errorStatus);
      return;
    });
  jest
    .spyOn(appContext.mainProcessClient, 'subscribeToAgentUpdate')
    .mockImplementation((rootClusterUri, listener) => {
      eventEmitter.on('', listener);
      return { cleanup: () => eventEmitter.off('', listener) };
    });

  const { result } = renderHook(() => useConnectMyComputerContext(), {
    wrapper: ({ children }) => (
      <MockAppContextProvider appContext={appContext}>
        <WorkspaceContextProvider value={null}>
          <ConnectMyComputerContextProvider rootClusterUri={rootCluster.uri}>
            {children}
          </ConnectMyComputerContextProvider>
        </WorkspaceContextProvider>
      </MockAppContextProvider>
    ),
  });

  let error: Error;
  await act(async () => {
    [, error] = await result.current.startAgent();
  });
  expect(result.current.currentAction).toStrictEqual({
    kind: 'start',
    attempt: makeErrorAttempt(new AgentProcessError()),
    agentProcessState: {
      status: 'error',
      message: 'ENOENT',
    },
    timeoutLogs: '',
  });
  expect(error).toBeInstanceOf(AgentProcessError);
});

test('starting the agent flips the workspace autoStart flag to true', async () => {
  const { appContext, rootCluster } = getMocksWithConnectMyComputerEnabled();

  jest
    .spyOn(appContext.connectMyComputerService, 'waitForNodeToJoin')
    .mockResolvedValue(makeServer());

  const { result } = renderHook(() => useConnectMyComputerContext(), {
    wrapper: ({ children }) => (
      <MockAppContextProvider appContext={appContext}>
        <WorkspaceContextProvider value={null}>
          <ConnectMyComputerContextProvider rootClusterUri={rootCluster.uri}>
            {children}
          </ConnectMyComputerContextProvider>
        </WorkspaceContextProvider>
      </MockAppContextProvider>
    ),
  });

  await act(async () => {
    const [, error] = await result.current.startAgent();
    expect(error).toBeFalsy();
  });

  expect(
    appContext.workspacesService.getConnectMyComputerAutoStart(rootCluster.uri)
  ).toBeTruthy();
});

test('killing the agent flips the workspace autoStart flag to false', async () => {
  const { appContext, rootCluster } = getMocksWithConnectMyComputerEnabled();

  const { result } = renderHook(() => useConnectMyComputerContext(), {
    wrapper: ({ children }) => (
      <MockAppContextProvider appContext={appContext}>
        <WorkspaceContextProvider value={null}>
          <ConnectMyComputerContextProvider rootClusterUri={rootCluster.uri}>
            {children}
          </ConnectMyComputerContextProvider>
        </WorkspaceContextProvider>
      </MockAppContextProvider>
    ),
  });

  await act(() => result.current.killAgent());

  expect(
    appContext.workspacesService.getConnectMyComputerAutoStart(rootCluster.uri)
  ).toBeFalsy();
});

test('starts the agent automatically if the workspace autoStart flag is true', async () => {
  const { appContext, rootCluster } = getMocksWithConnectMyComputerEnabled();

  const eventEmitter = new EventEmitter();
  let currentAgentProcessState: AgentProcessState = {
    status: 'not-started',
  };
  jest
    .spyOn(appContext.mainProcessClient, 'getAgentState')
    .mockImplementation(() => currentAgentProcessState);
  jest
    .spyOn(appContext.connectMyComputerService, 'isAgentConfigFileCreated')
    .mockResolvedValue(true);
  jest
    .spyOn(appContext.connectMyComputerService, 'runAgent')
    .mockImplementation(async () => {
      currentAgentProcessState = { status: 'running' };
      eventEmitter.emit('', currentAgentProcessState);
    });
  jest
    .spyOn(appContext.connectMyComputerService, 'waitForNodeToJoin')
    .mockResolvedValue(makeServer());
  jest.spyOn(appContext.connectMyComputerService, 'downloadAgent');
  jest
    .spyOn(appContext.mainProcessClient, 'subscribeToAgentUpdate')
    .mockImplementation((rootClusterUri, listener) => {
      eventEmitter.on('', listener);
      return { cleanup: () => eventEmitter.off('', listener) };
    });

  jest
    .spyOn(appContext.workspacesService, 'getConnectMyComputerAutoStart')
    .mockReturnValue(true);

  const { result, waitFor } = renderHook(() => useConnectMyComputerContext(), {
    wrapper: ({ children }) => (
      <MockAppContextProvider appContext={appContext}>
        <WorkspaceContextProvider value={null}>
          <ConnectMyComputerContextProvider rootClusterUri={rootCluster.uri}>
            {children}
          </ConnectMyComputerContextProvider>
        </WorkspaceContextProvider>
      </MockAppContextProvider>
    ),
  });

  await waitFor(
    () =>
      result.current.currentAction.kind === 'observe-process' &&
      result.current.currentAction.agentProcessState.status === 'running'
  );

  expect(
    appContext.connectMyComputerService.downloadAgent
  ).toHaveBeenCalledTimes(1);
  expect(appContext.connectMyComputerService.runAgent).toHaveBeenCalledTimes(1);
});

describe('canUse', () => {
  const cases = [
    {
      name: 'should be true when the user has permissions and the feature flag is enabled',
      hasPermissions: true,
      isFeatureFlagEnabled: true,
      isAgentConfigured: false,
      expected: true,
    },
    {
      name: 'should be true when the user does not have permissions, but the agent has been configured and the feature flag is enabled',
      hasPermissions: false,
      isFeatureFlagEnabled: true,
      isAgentConfigured: true,
      expected: true,
    },
    {
      name: 'should be false when the user does not have permissions, the agent has not been configured and the feature flag is enabled',
      hasPermissions: false,
      isAgentConfigured: false,
      isFeatureFlagEnabled: true,
      expected: false,
    },
    {
      name: 'should be false when the user has permissions and the agent is configured but the feature flag is disabled',
      hasPermissions: true,
      isAgentConfigured: true,
      isFeatureFlagEnabled: false,
      expected: false,
    },
  ];

  test.each(cases)(
    '$name',
    async ({
      hasPermissions,
      isAgentConfigured,
      isFeatureFlagEnabled,
      expected,
    }) => {
      const { appContext, rootCluster } =
        getMocksWithConnectMyComputerEnabled();
      // update Connect My Computer permissions
      appContext.clustersService.setState(draftState => {
        draftState.clusters.get(
          rootCluster.uri
        ).loggedInUser.acl.tokens.create = hasPermissions;
      });
      appContext.configService = createMockConfigService({
        'feature.connectMyComputer': isFeatureFlagEnabled,
      });
      const isAgentConfigFileCreated = Promise.resolve(isAgentConfigured);
      jest
        .spyOn(appContext.connectMyComputerService, 'isAgentConfigFileCreated')
        .mockReturnValue(isAgentConfigFileCreated);

      const { result } = renderHook(() => useConnectMyComputerContext(), {
        wrapper: ({ children }) => (
          <MockAppContextProvider appContext={appContext}>
            <WorkspaceContextProvider value={null}>
              <ConnectMyComputerContextProvider
                rootClusterUri={rootCluster.uri}
              >
                {children}
              </ConnectMyComputerContextProvider>
            </WorkspaceContextProvider>
          </MockAppContextProvider>
        ),
      });

      await act(() => isAgentConfigFileCreated);

      expect(result.current.canUse).toBe(expected);
    }
  );
});
