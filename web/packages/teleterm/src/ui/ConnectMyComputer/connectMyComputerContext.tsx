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
  createContext,
  FC,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react';

import { wait } from 'shared/utils/wait';

import { Attempt, makeSuccessAttempt, useAsync } from 'shared/hooks/useAsync';

import { RootClusterUri } from 'teleterm/ui/uri';
import { useAppContext } from 'teleterm/ui/appContextProvider';

import { Server } from 'teleterm/services/tshd/types';

import { assertUnreachable } from '../utils';

import { canUseConnectMyComputer } from './permissions';

import type {
  AgentProcessState,
  MainProcessClient,
} from 'teleterm/mainProcess/types';

export type CurrentAction =
  | {
      kind: 'download';
      attempt: Attempt<void>;
    }
  | {
      kind: 'start';
      attempt: Attempt<Server>;
      agentProcessState: AgentProcessState;
    }
  | {
      kind: 'observe-process';
      agentProcessState: AgentProcessState;
    }
  | {
      kind: 'kill';
      attempt: Attempt<void>;
    };

export interface ConnectMyComputerContext {
  canUse: boolean;
  currentAction: CurrentAction;
  agentProcessState: AgentProcessState;
  agentNode: Server | undefined;
  startAgent(): Promise<[Server, Error]>;
  downloadAgent(): Promise<[void, Error]>;
  downloadAgentAttempt: Attempt<void>;
  setDownloadAgentAttempt(attempt: Attempt<void>): void;
  downloadAndStartAgent(): Promise<void>;
  killAgent(): Promise<[void, Error]>;
  isAgentConfiguredAttempt: Attempt<boolean>;
  markAgentAsConfigured(): void;
}

const ConnectMyComputerContext = createContext<ConnectMyComputerContext>(null);

export const ConnectMyComputerContextProvider: FC<{
  rootClusterUri: RootClusterUri;
}> = props => {
  const {
    mainProcessClient,
    connectMyComputerService,
    clustersService,
    configService,
    workspacesService,
  } = useAppContext();
  clustersService.useState();

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

  const rootCluster = clustersService.findCluster(props.rootClusterUri);
  const canUse = useMemo(
    () =>
      canUseConnectMyComputer(
        rootCluster,
        configService,
        mainProcessClient.getRuntimeSettings()
      ),
    [configService, mainProcessClient, rootCluster]
  );

  const markAgentAsConfigured = useCallback(() => {
    setAgentConfiguredAttempt(makeSuccessAttempt(true));
  }, [setAgentConfiguredAttempt]);

  const [currentActionKind, setCurrentActionKind] =
    useState<CurrentAction['kind']>('observe-process');

  const [agentProcessState, setAgentProcessState] = useState<AgentProcessState>(
    () =>
      mainProcessClient.getAgentState({
        rootClusterUri: props.rootClusterUri,
      }) || {
        status: 'not-started',
      }
  );

  const [downloadAgentAttempt, downloadAgent, setDownloadAgentAttempt] =
    useAsync(
      useCallback(async () => {
        setCurrentActionKind('download');
        await connectMyComputerService.downloadAgent();
      }, [connectMyComputerService])
    );

  const [startAgentAttempt, startAgent] = useAsync(
    useCallback(async () => {
      setCurrentActionKind('start');
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
        setCurrentActionKind('observe-process');
        workspacesService.setConnectMyComputerAutoStart(
          props.rootClusterUri,
          true
        );
        return server;
      } catch (error) {
        // in case of any error kill the agent
        await connectMyComputerService.killAgent(props.rootClusterUri);
        throw error;
      } finally {
        abortController.abort();
      }
    }, [
      connectMyComputerService,
      mainProcessClient,
      props.rootClusterUri,
      workspacesService,
    ])
  );

  const downloadAndStartAgent = useCallback(async () => {
    const [, error] = await downloadAgent();
    if (error) {
      return;
    }
    await startAgent();
  }, [downloadAgent, startAgent]);

  const [killAgentAttempt, killAgent] = useAsync(
    useCallback(async () => {
      setCurrentActionKind('kill');
      await connectMyComputerService.killAgent(props.rootClusterUri);
      setCurrentActionKind('observe-process');
      workspacesService.setConnectMyComputerAutoStart(
        props.rootClusterUri,
        false
      );
    }, [connectMyComputerService, props.rootClusterUri, workspacesService])
  );

  useEffect(() => {
    const { cleanup } = mainProcessClient.subscribeToAgentUpdate(
      props.rootClusterUri,
      setAgentProcessState
    );
    return cleanup;
  }, [mainProcessClient, props.rootClusterUri]);

  let currentAction: CurrentAction;
  const kind = currentActionKind;

  switch (kind) {
    case 'download': {
      currentAction = { kind, attempt: downloadAgentAttempt };
      break;
    }
    case 'start': {
      currentAction = { kind, attempt: startAgentAttempt, agentProcessState };
      break;
    }
    case 'observe-process': {
      currentAction = { kind, agentProcessState };
      break;
    }
    case 'kill': {
      currentAction = { kind, attempt: killAgentAttempt };
      break;
    }
    default: {
      assertUnreachable(kind);
    }
  }

  useEffect(() => {
    // This call checks if the agent is configured even if the user does not have access to Connect My Computer.
    // Unfortunately, we cannot call it only if `canUse === true`, because to resolve `canUse` value
    // we need to fetch some data from the auth server which takes time.
    // This doesn't work for us, because the information if the agent is configured is needed immediately -
    // based on this we replace the setup document with the status document.
    // If we had waited for `canUse` to become true, the user might have seen a setup document
    // which would have been replaced by the other document after 1-2 seconds.
    if (isAgentConfiguredAttempt.status === '') {
      checkIfAgentIsConfigured();
    }
  }, [checkIfAgentIsConfigured, isAgentConfiguredAttempt.status]);

  const isAgentConfigured =
    isAgentConfiguredAttempt.status === 'success' &&
    isAgentConfiguredAttempt.data;
  const agentIsNotStarted =
    currentAction.kind === 'observe-process' &&
    currentAction.agentProcessState.status === 'not-started';

  useEffect(() => {
    const shouldAutoStartAgent =
      isAgentConfigured &&
      canUse &&
      workspacesService.getConnectMyComputerAutoStart(props.rootClusterUri) &&
      agentIsNotStarted;
    if (shouldAutoStartAgent) {
      downloadAndStartAgent();
    }
  }, [
    canUse,
    downloadAndStartAgent,
    agentIsNotStarted,
    isAgentConfigured,
    props.rootClusterUri,
    workspacesService,
  ]);

  return (
    <ConnectMyComputerContext.Provider
      value={{
        canUse,
        currentAction,
        agentProcessState,
        agentNode: startAgentAttempt.data,
        killAgent,
        startAgent,
        downloadAgent,
        downloadAgentAttempt,
        setDownloadAgentAttempt,
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
        // TODO(ravicious): 'error' should not be considered a separate process state. See the
        // comment above the 'error' status definition.
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
