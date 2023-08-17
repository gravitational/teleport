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
} from 'react';

import { wait } from 'shared/utils/wait';

import { RootClusterUri } from 'teleterm/ui/uri';
import { useAppContext } from 'teleterm/ui/appContextProvider';

import type {
  AgentProcessState,
  MainProcessClient,
} from 'teleterm/mainProcess/types';

export interface ConnectMyComputerContext {
  state: AgentProcessState;
  runAgentAndWaitForNodeToJoin(): Promise<void>;
}

const ConnectMyComputerContext = createContext<ConnectMyComputerContext>(null);

export const ConnectMyComputerContextProvider: FC<{
  rootClusterUri: RootClusterUri;
}> = props => {
  const { mainProcessClient, connectMyComputerService } = useAppContext();
  const [agentState, setAgentState] = useState<AgentProcessState>(
    () =>
      mainProcessClient.getAgentState({
        rootClusterUri: props.rootClusterUri,
      }) || {
        status: 'not-started',
      }
  );

  const runAgentAndWaitForNodeToJoin = useCallback(async () => {
    await connectMyComputerService.runAgent(props.rootClusterUri);

    // TODO(gzdunek): Replace with waiting for the node to join.
    const waitForNodeToJoin = wait(1_000);

    await Promise.race([
      waitForNodeToJoin,
      waitForAgentProcessErrors(mainProcessClient, props.rootClusterUri),
    ]);
  }, [connectMyComputerService, mainProcessClient, props.rootClusterUri]);

  useEffect(() => {
    const { cleanup } = mainProcessClient.subscribeToAgentUpdate(
      props.rootClusterUri,
      state => setAgentState(state)
    );
    return cleanup;
  }, [mainProcessClient, props.rootClusterUri]);

  return (
    <ConnectMyComputerContext.Provider
      value={{
        state: agentState,
        runAgentAndWaitForNodeToJoin,
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
 * If none of them happen within 20 seconds, the promise resolves.
 */
async function waitForAgentProcessErrors(
  mainProcessClient: MainProcessClient,
  rootClusterUri: RootClusterUri
) {
  let cleanupFn: () => void;

  try {
    const errorPromise = new Promise((_, reject) => {
      const { cleanup } = mainProcessClient.subscribeToAgentUpdate(
        rootClusterUri,
        agentProcessState => {
          const error = isProcessInErrorOrExitState(agentProcessState);
          if (error) {
            reject(error);
          }
        }
      );

      // the state may have changed before we started listening, we have to check the current state
      const agentProcessState = mainProcessClient.getAgentState({
        rootClusterUri,
      });
      const error = isProcessInErrorOrExitState(agentProcessState);
      if (error) {
        reject(error);
      }

      cleanupFn = cleanup;
    });
    await Promise.race([errorPromise, wait(20_000)]);
  } finally {
    cleanupFn();
  }
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
        `Agent process exited with ${codeOrSignal}.`,
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
