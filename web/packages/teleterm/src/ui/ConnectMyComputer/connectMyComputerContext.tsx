/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, {
  useContext,
  FC,
  createContext,
  useState,
  useEffect,
  useCallback,
  useRef,
} from 'react';

import { wait } from 'shared/utils/wait';

import { Attempt, makeSuccessAttempt, useAsync } from 'shared/hooks/useAsync';

import { RootClusterUri } from 'teleterm/ui/uri';
import { useAppContext } from 'teleterm/ui/appContextProvider';

import { Server } from 'teleterm/services/tshd/types';

import type {
  AgentProcessState,
  MainProcessClient,
} from 'teleterm/mainProcess/types';

export type AgentState = AgentProcessState;

export interface ConnectMyComputerContext {
  agentState: AgentState;
  agentNode: Server;
  runAgentAndWaitForNodeToJoin(): Promise<void>;
  downloadAndRun(): Promise<[void, Error]>;
  kill(): Promise<[void, Error]>;
  isAgentConfiguredAttempt: Attempt<boolean>;
  markAgentAsConfigured(): void;
  lifecycleActionAttempt: Attempt<void>;
}

const ConnectMyComputerContext = createContext<ConnectMyComputerContext>(null);

export const ConnectMyComputerContextProvider: FC<{
  rootClusterUri: RootClusterUri;
}> = props => {
  const { mainProcessClient, connectMyComputerService } = useAppContext();
  const [
    isAgentConfiguredAttempt,
    checkIfAgentIsConfigured,
    setAgentConfiguredAttempt,
  ] = useAsync(
    useCallback(
      () =>
        connectMyComputerService.isAgentConfigFileCreated(props.rootClusterUri),
      [connectMyComputerService, props.rootClusterUri]
    )
  );

  const [agentProcessState, setAgentProcessState] = useState<AgentProcessState>(
    () =>
      mainProcessClient.getAgentState({
        rootClusterUri: props.rootClusterUri,
      }) || {
        status: 'not-started',
      }
  );

  const [runAgentAndWaitForNodeToJoinAttempt, runAgentAndWaitForNodeToJoin] =
    useAsync(
      useCallback(async () => {
        await connectMyComputerService.runAgent(props.rootClusterUri);

        const abortController = new AbortController();
        return new Promise<Server>((resolve, reject) => {
          //TODO(gzdunek): Do we need to kill the agent if any of the following promises fail?
          Promise.race([
            connectMyComputerService
              .waitForNodeToJoin(props.rootClusterUri, abortController.signal)
              .then(resolve),
            waitForAgentProcessErrors(
              mainProcessClient,
              props.rootClusterUri,
              abortController.signal
            ).then(reject),
            wait(20000, abortController.signal).then(() =>
              reject(
                new Error(
                  'The agent did not manage to join the cluster within 20 seconds.'
                )
              )
            ),
          ]).finally(() => abortController.abort());
        });
      }, [connectMyComputerService, mainProcessClient, props.rootClusterUri])
    );

  const [lifecycleActionAttempt, runLifecycleActionAttempt] = useAsync(
    async (cb: () => Promise<void>) => await cb()
  );

  const downloadAndRun = useCallback(
    async () =>
      runLifecycleActionAttempt(async () => {
        await connectMyComputerService.downloadAgent(props.rootClusterUri);
        await runAgentAndWaitForNodeToJoin();
      }),
    [
      connectMyComputerService,
      props.rootClusterUri,
      runAgentAndWaitForNodeToJoin,
      runLifecycleActionAttempt,
    ]
  );

  const kill = useCallback(
    async () =>
      runLifecycleActionAttempt(() =>
        connectMyComputerService.killAgent(props.rootClusterUri)
      ),
    [connectMyComputerService, props.rootClusterUri, runLifecycleActionAttempt]
  );

  const markAgentAsConfigured = useCallback(() => {
    setAgentConfiguredAttempt(makeSuccessAttempt(true));
  }, [setAgentConfiguredAttempt]);

  useEffect(() => {
    const { cleanup } = mainProcessClient.subscribeToAgentUpdate(
      props.rootClusterUri,
      state => setAgentProcessState(state)
    );
    return cleanup;
  }, [mainProcessClient, props.rootClusterUri]);

  useEffect(() => {
    checkIfAgentIsConfigured();
  }, [checkIfAgentIsConfigured]);

  const computedState = computeAgentState(
    agentProcessState,
    waitForNodeToJoinAttempt
  );

  return (
    <ConnectMyComputerContext.Provider
      value={{
        agentState: computedState,
        runAgentAndWaitForNodeToJoin,
        agentNode: waitForNodeToJoinAttempt.data,
        lifecycleActionAttempt,
        kill,
        downloadAndRun,
        markAgentAsConfigured,
        isAgentConfiguredAttempt,
      }}
      children={props.children}
    />
  );
};

export const useConnectMyComputerContext = () => {
  const context = useContext(ConnectMyComputerContext);

  if (!context) {
    throw new Error(
      'ConnectMyComputerContext requires ConnectMyComputerContextProvider context.'
    );
  }

  return context;
};

/**
 * Waits for `error` and `exit` events from the agent process.
 */
function waitForAgentProcessErrors(
  mainProcessClient: MainProcessClient,
  rootClusterUri: RootClusterUri,
  abortSignal: AbortSignal
) {
  return new Promise((resolve, reject) => {
    const { cleanup } = mainProcessClient.subscribeToAgentUpdate(
      rootClusterUri,
      agentProcessState => {
        const error = isProcessInErrorOrExitState(agentProcessState);
        if (error) {
          resolve(error);
          cleanup();
        }
      }
    );
    abortSignal.onabort = () => {
      cleanup();
      reject();
    };

    // the state may have changed before we started listening, we have to check the current state
    const agentProcessState = mainProcessClient.getAgentState({
      rootClusterUri,
    });
    const error = isProcessInErrorOrExitState(agentProcessState);
    if (error) {
      resolve(error);
      cleanup();
    }
  });
}

function isProcessInErrorOrExitState(
  agentProcessState: AgentProcessState
): Error | undefined {
  if (agentProcessState.status === 'exited') {
    const { code, signal } = agentProcessState;
    const codeOrSignal = [
      // code can be 0, so we cannot just check it the same way as the signal.
      code != null && `code ${code}`,
      signal && `signal ${signal}`,
    ]
      .filter(Boolean)
      .join(' ');

    return new Error(
      [
        `Agent process failed to start - the process exited with ${codeOrSignal}.`,
        agentProcessState.stackTrace,
      ]
        .filter(Boolean)
        .join('\n')
    );
  }
  if (agentProcessState.status === 'error') {
    return new Error(
      ['Agent process failed to start.', agentProcessState.message].join('\n')
    );
  }
}

function computeAgentState(
  agentState: AgentProcessState,
  nodeReadyAttempt: Attempt<Server>
  // downloadAgentAttempt: Attempt<void>,
): AgentState {
  if (nodeReadyAttempt.status === 'processing') {
    return { status: 'starting' };
  }

  return agentState;
}
