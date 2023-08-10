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

import { Attempt, makeSuccessAttempt, useAsync } from 'shared/hooks/useAsync';

import { RootClusterUri } from 'teleterm/ui/uri';
import { useAppContext } from 'teleterm/ui/appContextProvider';

import { Server } from 'teleterm/services/tshd/types';

import type {
  AgentProcessState,
  MainProcessClient,
} from 'teleterm/mainProcess/types';

export type AgentState =
  | {
      status: 'process-not-started';
    }
  | {
      status: 'process-running';
    }
  | {
      status: 'process-exited';
      code: number | null;
      signal: NodeJS.Signals | null;
      exitedSuccessfully: boolean;
      /** Fragment of a stack trace when the process did not exit successfully. */
      stackTrace?: string;
    }
  | {
      status: 'process-error';
      message: string;
    }
  | {
      status: 'downloading';
    }
  | {
      status: 'download-error';
      message: string;
    }
  | {
      status: 'starting';
    }
  | {
      status: 'join-error';
      message: string;
    }
  | {
      status: 'killing';
    }
  | {
      status: 'kill-error';
      message: string;
    }
  | {
      status: '';
    };

type CurrentAttempt = 'download' | 'start' | 'process' | 'kill';

export interface ConnectMyComputerContext {
  agentState: AgentState;
  agentNode: Server | undefined;
  startAgent(): Promise<[Server, Error]>;
  downloadAgent(): Promise<[void, Error]>;
  downloadAndStartAgent(): Promise<void>;
  killAgent(): Promise<[void, Error]>;
  isAgentConfiguredAttempt: Attempt<boolean>;
  markAgentAsConfigured(): void;
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

  const markAgentAsConfigured = useCallback(() => {
    setAgentConfiguredAttempt(makeSuccessAttempt(true));
  }, [setAgentConfiguredAttempt]);

  const [currentAttempt, setCurrentAttempt] =
    useState<CurrentAttempt>('process');

  const [agentProcessState, setAgentProcessState] = useState<AgentProcessState>(
    () =>
      mainProcessClient.getAgentState({
        rootClusterUri: props.rootClusterUri,
      }) || {
        status: 'not-started',
      }
  );

  const [downloadAgentAttempt, downloadAgent] = useAsync(
    useCallback(async () => {
      setCurrentAttempt('download');
      await connectMyComputerService.downloadAgent();
    }, [connectMyComputerService])
  );

  const [startAgentAttempt, startAgent] = useAsync(
    useCallback(async () => {
      setCurrentAttempt('start');
      await connectMyComputerService.runAgent(props.rootClusterUri);

      const abortController = new AbortController();
      try {
        const server = await Promise.race([
          connectMyComputerService.waitForNodeToJoin(
            props.rootClusterUri,
            abortController.signal
          ),
          throwOnAgentProcessErrors(
            mainProcessClient,
            props.rootClusterUri,
            abortController.signal
          ),
          wait(20_000, abortController.signal).then(() => {
            throw new Error(
              'The agent did not manage to join the cluster within 20 seconds.'
            );
          }),
        ]);
        setCurrentAttempt('process');
        return server;
      } catch (error) {
        // in case of any error kill the agent
        await connectMyComputerService.killAgent(props.rootClusterUri);
        throw error;
      } finally {
        abortController.abort();
      }
    }, [connectMyComputerService, mainProcessClient, props.rootClusterUri])
  );

  const downloadAndStartAgent = async () => {
    const [, error] = await downloadAgent();
    if (error) {
      return;
    }
    await startAgent();
  };

  const [killAgentAttempt, killAgent] = useAsync(
    useCallback(async () => {
      setCurrentAttempt('kill');
      await connectMyComputerService.killAgent(props.rootClusterUri);
      setCurrentAttempt('process');
    }, [connectMyComputerService, props.rootClusterUri])
  );

  useEffect(() => {
    const { cleanup } = mainProcessClient.subscribeToAgentUpdate(
      props.rootClusterUri,
      setAgentProcessState
    );
    return cleanup;
  }, [mainProcessClient, props.rootClusterUri]);

  useEffect(() => {
    checkIfAgentIsConfigured();
  }, [checkIfAgentIsConfigured]);

  const computedAgentState = computeAgentState({
    currentAttempt,
    downloadAgentAttempt,
    startAgentAttempt,
    agentProcessStateAttempt: makeSuccessAttempt(agentProcessState),
    killAgentAttempt,
  });

  return (
    <ConnectMyComputerContext.Provider
      value={{
        agentState: computedAgentState,
        agentNode: startAgentAttempt.data,
        killAgent,
        startAgent,
        downloadAgent,
        downloadAndStartAgent,
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
 * Returns agent state based on multiple sources.
 * Not all possible cases are handled, for example `download.success` -
 * it is expected that the next state that the user should see is `start.processing`.
 */
function computeAgentState({
  currentAttempt,
  downloadAgentAttempt,
  startAgentAttempt,
  agentProcessStateAttempt,
  killAgentAttempt,
}: {
  currentAttempt: CurrentAttempt;
  downloadAgentAttempt: Attempt<void>;
  startAgentAttempt: Attempt<Server>;
  agentProcessStateAttempt: Attempt<AgentProcessState>;
  killAgentAttempt: Attempt<void>;
}): AgentState {
  const agentProcessState = agentProcessStateAttempt.data;
  switch (currentAttempt) {
    case 'download': {
      switch (downloadAgentAttempt.status) {
        case 'processing': {
          return { status: 'downloading' };
        }
        case 'error': {
          return {
            status: 'download-error',
            message: downloadAgentAttempt.statusText,
          };
        }
      }
      break;
    }
    case 'start': {
      switch (startAgentAttempt.status) {
        case 'processing': {
          return { status: 'starting' };
        }
        case 'error': {
          if (startAgentAttempt.statusText === AgentProcessError.name) {
            if (agentProcessState.status === 'exited') {
              return {
                ...agentProcessState,
                status: 'process-exited',
              };
            }
            if (agentProcessState.status === 'error') {
              return {
                ...agentProcessState,
                status: 'process-error',
              };
            }
          }
          return {
            status: 'join-error',
            message: startAgentAttempt.statusText,
          };
        }
      }
      break;
    }
    case 'process': {
      switch (agentProcessStateAttempt.status) {
        case 'success': {
          if (agentProcessState.status === 'exited') {
            return {
              ...agentProcessState,
              status: 'process-exited',
            };
          }
          if (agentProcessState.status === 'error') {
            return {
              ...agentProcessState,
              status: 'process-error',
            };
          }
          if (agentProcessState.status === 'running') {
            return {
              ...agentProcessState,
              status: 'process-running',
            };
          }
          if (agentProcessState.status === 'not-started') {
            return {
              ...agentProcessState,
              status: 'process-not-started',
            };
          }
        }
      }
      break;
    }
    case 'kill': {
      switch (killAgentAttempt.status) {
        case 'processing': {
          return { status: 'killing' };
        }
        case 'error': {
          return {
            status: 'kill-error',
            message: killAgentAttempt.statusText,
          };
        }
      }
      break;
    }
  }
  return { status: '' };
}

/**
 * Waits for `error` and `exit` events from the agent process and throws when they occur.
 */
function throwOnAgentProcessErrors(
  mainProcessClient: MainProcessClient,
  rootClusterUri: RootClusterUri,
  abortSignal: AbortSignal
): Promise<never> {
  return new Promise((_, reject) => {
    const rejectOnError = (agentProcessState: AgentProcessState) => {
      if (
        agentProcessState.status === 'exited' ||
        agentProcessState.status === 'error'
      ) {
        reject(new AgentProcessError());
        cleanup();
      }
    };

    const { cleanup } = mainProcessClient.subscribeToAgentUpdate(
      rootClusterUri,
      rejectOnError
    );
    abortSignal.onabort = () => {
      cleanup();
      reject(
        new DOMException('throwOnAgentProcessErrors was aborted', 'AbortError')
      );
    };

    // the state may have changed before we started listening, we have to check the current state
    rejectOnError(
      mainProcessClient.getAgentState({
        rootClusterUri,
      })
    );
  });
}

export class AgentProcessError extends Error {
  constructor() {
    super('AgentProcessError');
    this.name = 'AgentProcessError';
  }
}
