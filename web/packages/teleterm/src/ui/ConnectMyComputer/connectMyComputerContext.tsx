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

import {
  createContext,
  FC,
  PropsWithChildren,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react';

import {
  Attempt,
  makeEmptyAttempt,
  makeSuccessAttempt,
  useAsync,
} from 'shared/hooks/useAsync';
import { wait } from 'shared/utils/wait';

import type {
  AgentProcessState,
  MainProcessClient,
} from 'teleterm/mainProcess/types';
import {
  cloneAbortSignal,
  isTshdRpcError,
} from 'teleterm/services/tshd/cloneableClient';
import { Server } from 'teleterm/services/tshd/types';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useResourcesContext } from 'teleterm/ui/DocumentCluster/resourcesContext';
import { useLogger } from 'teleterm/ui/hooks/useLogger';
import { RootClusterUri, routing } from 'teleterm/ui/uri';

import { assertUnreachable, retryWithRelogin } from '../utils';
import { ConnectMyComputerAccess, getConnectMyComputerAccess } from './access';
import {
  AgentCompatibility,
  checkAgentCompatibility,
} from './CompatibilityPromise';

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
    }
  | {
      kind: 'remove';
      attempt: Attempt<void>;
    };

export interface ConnectMyComputerContext {
  /**
   * canUse describes whether the user should be allowed to use Connect My Computer.
   * This is true either when the user has access to Connect My Computer or they have already set up
   * the agent.
   *
   * The second case is there to protect from a scenario where a malicious admin lets the user set
   * up the agent but then revokes their access for creating tokens. Without checking if the agent
   * was already set up, the user would have lost control over the agent.
   * https://github.com/gravitational/teleport/blob/master/rfd/0133-connect-my-computer.md#access-to-ui-and-autostart
   */
  canUse: boolean;
  /**
   * access describes whether the user has the necessary requirements to use Connect My Computer. It
   * does not account for the agent being already set up. Thus it's mostly useful in scenarios where
   * this has been already accounted for, for example when showing an alert about insufficient
   * permissions in the setup step.
   */
  access: ConnectMyComputerAccess;
  currentAction: CurrentAction;
  agentProcessState: AgentProcessState;
  agentNode: Server | undefined;
  startAgent(): Promise<[Server, Error]>;
  downloadAgent(): Promise<[void, Error]>;
  downloadAgentAttempt: Attempt<void>;
  setDownloadAgentAttempt(attempt: Attempt<void>): void;
  downloadAndStartAgent(): Promise<void>;
  killAgent(): Promise<[void, Error]>;
  removeAgent(): Promise<[void, Error]>;
  isAgentConfiguredAttempt: Attempt<boolean>;
  markAgentAsConfigured(): void;
  markAgentAsNotConfigured(): void;
  agentCompatibility: AgentCompatibility;
}

const ConnectMyComputerContext = createContext<ConnectMyComputerContext>(null);

export const ConnectMyComputerContextProvider: FC<
  PropsWithChildren<{
    rootClusterUri: RootClusterUri;
  }>
> = ({ rootClusterUri, children }) => {
  const logger = useLogger('connectMyComputerContext');
  const ctx = useAppContext();
  const {
    mainProcessClient,
    connectMyComputerService,
    clustersService,
    workspacesService,
    usageService,
  } = ctx;
  const { requestResourcesRefresh } = useResourcesContext(rootClusterUri);
  clustersService.useState();

  const [
    isAgentConfiguredAttempt,
    checkIfAgentIsConfigured,
    setAgentConfiguredAttempt,
  ] = useAsync(
    useCallback(
      () => connectMyComputerService.isAgentConfigFileCreated(rootClusterUri),
      [connectMyComputerService, rootClusterUri]
    )
  );
  const isAgentConfigured =
    isAgentConfiguredAttempt.status === 'success' &&
    isAgentConfiguredAttempt.data;

  const rootCluster = clustersService.findCluster(rootClusterUri);
  const { loggedInUser } = rootCluster;

  const access = useMemo(
    () =>
      getConnectMyComputerAccess(
        loggedInUser,
        mainProcessClient.getRuntimeSettings()
      ),
    [loggedInUser, mainProcessClient]
  );
  const canUse = access.status === 'ok' || isAgentConfigured;

  const agentCompatibility = useMemo(
    () =>
      checkAgentCompatibility(
        rootCluster.proxyVersion,
        mainProcessClient.getRuntimeSettings()
      ),
    [mainProcessClient, rootCluster]
  );

  const [currentActionKind, setCurrentActionKind] =
    useState<CurrentAction['kind']>('observe-process');

  const [agentProcessState, setAgentProcessState] = useState<AgentProcessState>(
    () =>
      mainProcessClient.getAgentState({
        rootClusterUri,
      }) || {
        status: 'not-started',
      }
  );

  const checkCompatibility = useCallback(() => {
    if (agentCompatibility !== 'compatible') {
      throw new AgentCompatibilityError(agentCompatibility);
    }
  }, [agentCompatibility]);

  const [downloadAgentAttempt, downloadAgent, setDownloadAgentAttempt] =
    useAsync(
      useCallback(async () => {
        setCurrentActionKind('download');
        checkCompatibility();
        await connectMyComputerService.downloadAgent();
      }, [connectMyComputerService, checkCompatibility])
    );

  const [startAgentAttempt, startAgent] = useAsync(
    useCallback(async () => {
      setCurrentActionKind('start');

      checkCompatibility();

      await connectMyComputerService.runAgent(rootClusterUri);

      const abortController = new AbortController();
      try {
        const server = await Promise.race([
          connectMyComputerService.waitForNodeToJoin(
            rootClusterUri,
            cloneAbortSignal(abortController.signal)
          ),
          throwOnAgentProcessErrors(
            mainProcessClient,
            rootClusterUri,
            abortController.signal
          ),
          wait(20_000, abortController.signal).then(() => {
            const logs = mainProcessClient.getAgentLogs({ rootClusterUri });
            throw new NodeWaitJoinTimeout(logs);
          }),
        ]);
        setCurrentActionKind('observe-process');
        workspacesService.setConnectMyComputerAutoStart(rootClusterUri, true);
        usageService.captureConnectMyComputerAgentStart(rootClusterUri);
        return server;
      } catch (error) {
        // in case of any error kill the agent
        await connectMyComputerService.killAgent(rootClusterUri);
        throw error;
      } finally {
        abortController.abort();
      }
    }, [
      connectMyComputerService,
      mainProcessClient,
      rootClusterUri,
      usageService,
      workspacesService,
      checkCompatibility,
    ])
  );

  const downloadAndStartAgent = useCallback(async () => {
    let [, error] = await downloadAgent();
    if (error) {
      throw error;
    }
    [, error] = await startAgent();
    if (error) {
      throw error;
    }
  }, [downloadAgent, startAgent]);

  const [killAgentAttempt, killAgent] = useAsync(
    useCallback(async () => {
      setCurrentActionKind('kill');

      await connectMyComputerService.killAgent(rootClusterUri);

      setCurrentActionKind('observe-process');
      workspacesService.setConnectMyComputerAutoStart(rootClusterUri, false);
    }, [connectMyComputerService, rootClusterUri, workspacesService])
  );

  const markAgentAsConfigured = useCallback(() => {
    setAgentConfiguredAttempt(makeSuccessAttempt(true));
  }, [setAgentConfiguredAttempt]);

  const markAgentAsNotConfigured = useCallback(() => {
    setDownloadAgentAttempt(makeEmptyAttempt());
    setAgentConfiguredAttempt(makeSuccessAttempt(false));
  }, [setAgentConfiguredAttempt, setDownloadAgentAttempt]);

  const removeConnections = useCallback(async () => {
    const { rootClusterId } = routing.parseClusterUri(rootClusterUri).params;
    let nodeName: string;
    try {
      nodeName =
        await connectMyComputerService.getConnectMyComputerNodeName(
          rootClusterUri
        );
    } catch (error) {
      if (isTshdRpcError(error, 'NOT_FOUND')) {
        return;
      }
      throw error;
    }
    const nodeUri = routing.getServerUri({ rootClusterId, serverId: nodeName });
    await ctx.connectionTracker.disconnectAndRemoveItemsBelongingToResource(
      nodeUri
    );
  }, [connectMyComputerService, ctx.connectionTracker, rootClusterUri]);

  const [removeAgentAttempt, removeAgent] = useAsync(
    useCallback(async () => {
      // killAgent sets the current action to 'kill'.
      const [, error] = await killAgent();
      if (error) {
        throw error;
      }

      setCurrentActionKind('remove');

      let hasNodeRemovalSucceeded = true;
      try {
        await retryWithRelogin(ctx, rootClusterUri, () =>
          ctx.connectMyComputerService.removeConnectMyComputerNode(
            rootClusterUri
          )
        );
      } catch (error) {
        // Swallow all errors. Even if the cluster does not respond or responds with an error, it
        // should be possible to remove the agent.
        logger.warn(
          'Could not remove the Connect My Computer node in the cluster',
          error
        );
        hasNodeRemovalSucceeded = false;
      }

      if (hasNodeRemovalSucceeded) {
        requestResourcesRefresh();
      }

      ctx.notificationsService.notifyInfo(
        hasNodeRemovalSucceeded
          ? 'The agent has been removed.'
          : {
              title: 'The agent has been removed.',
              description:
                'The corresponding server may still be visible in the cluster for a few more minutes until it gets purged from the cache.',
            }
      );

      // We have to remove connections before removing the agent directory, because
      // we get the node UUID from the that directory.
      //
      // Theoretically, removing connections only at this stage means that if there are active
      // connections from the app at the time of killing the agent above, the shutdown of the agent
      // will take a couple of extra seconds while the agent waits for the connections to close.
      // However, we'd have to remove the connections before calling `killAgent` above and this
      // messes up error handling somewhat. `removeConnections` would have to be executed after the
      // current action is set to 'kill' so that any errors thrown by `removeConnections` are
      // correctly reported in the UI.
      //
      // Otherwise, if `removeConnections` was called before `killAgent` and the function threw an
      // error, it'd simply be swallowed. It'd be shown once the current action is set to 'remove',
      // but this would never happen because of the error.
      await removeConnections();
      ctx.workspacesService.removeConnectMyComputerState(rootClusterUri);
      await ctx.connectMyComputerService.removeAgentDirectory(rootClusterUri);

      markAgentAsNotConfigured();
    }, [
      ctx,
      killAgent,
      markAgentAsNotConfigured,
      removeConnections,
      rootClusterUri,
      requestResourcesRefresh,
      logger,
    ])
  );

  useEffect(() => {
    const { cleanup } = mainProcessClient.subscribeToAgentUpdate(
      rootClusterUri,
      setAgentProcessState
    );
    return cleanup;
  }, [mainProcessClient, rootClusterUri]);

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
    case 'remove': {
      currentAction = { kind, attempt: removeAgentAttempt };
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
    // If we had waited for `canUse` to become true, the user might have seen a setup page
    // which would have been replaced by status page after 1-2 seconds.
    if (isAgentConfiguredAttempt.status === '') {
      checkIfAgentIsConfigured();
    }
  }, [checkIfAgentIsConfigured, isAgentConfiguredAttempt.status]);

  const agentIsNotStarted =
    currentAction.kind === 'observe-process' &&
    currentAction.agentProcessState.status === 'not-started';
  const isAgentCompatibilityKnown = agentCompatibility !== 'unknown';

  useEffect(() => {
    const shouldAutoStartAgent =
      isAgentConfigured &&
      canUse &&
      // Agent compatibility is known only after we fetch full cluster details, so we have to wait
      // for that until we attempt to autostart the agent. Otherwise startAgent would return an
      // error.
      isAgentCompatibilityKnown &&
      workspacesService.getConnectMyComputerAutoStart(rootClusterUri) &&
      agentIsNotStarted;

    if (shouldAutoStartAgent) {
      (async () => {
        try {
          await downloadAndStartAgent();
        } catch {
          // Turn off autostart if it fails, otherwise the user wouldn't be able to turn it off by
          // themselves.
          workspacesService.setConnectMyComputerAutoStart(
            rootClusterUri,
            false
          );
        }
      })();
    }
  }, [
    canUse,
    downloadAndStartAgent,
    agentIsNotStarted,
    isAgentConfigured,
    rootClusterUri,
    workspacesService,
    isAgentCompatibilityKnown,
  ]);

  return (
    <ConnectMyComputerContext.Provider
      value={{
        canUse,
        access,
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
        markAgentAsNotConfigured,
        isAgentConfiguredAttempt,
        removeAgent,
        agentCompatibility,
      }}
      children={children}
    />
  );
};

export const useConnectMyComputerContext = () => {
  const context = useContext(ConnectMyComputerContext);

  if (!context) {
    throw new Error(
      'useConnectMyComputerContext must be used within a ConnectMyComputerContextProvider'
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
    abortSignal.addEventListener('abort', () => {
      cleanup();
      reject(
        new DOMException('throwOnAgentProcessErrors was aborted', 'AbortError')
      );
    });

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

export class NodeWaitJoinTimeout extends Error {
  constructor(public readonly logs: string) {
    super('NodeWaitJoinTimeout');
    this.name = 'NodeWaitJoinTimeout';
  }
}

export class AgentCompatibilityError extends Error {
  constructor(
    public readonly agentCompatibility: Exclude<
      AgentCompatibility,
      'compatible'
    >
  ) {
    let message: string;
    switch (agentCompatibility) {
      case 'incompatible': {
        message =
          'The agent version is not compatible with the cluster version';
        break;
      }
      case 'unknown': {
        message = 'The compatibility of the agent could not be established';
        break;
      }
      default: {
        throw assertUnreachable(agentCompatibility);
      }
    }
    super(message);
    this.name = 'AgentCompatibilityError';
  }
}
