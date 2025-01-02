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

import { useEffect, useLayoutEffect, useRef } from 'react';

import { AgentProcessState } from 'teleterm/mainProcess/types';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { ResourcesContextProvider } from 'teleterm/ui/DocumentCluster/resourcesContext';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';

import { ConnectMyComputerContextProvider } from './connectMyComputerContext';
import { NavigationMenu } from './NavigationMenu';

export default {
  title: 'Teleterm/ConnectMyComputer/NavigationMenu',
};

export function AgenNotConfigured() {
  return (
    <ShowState
      agentProcessState={{ status: 'not-started' }}
      isAgentConfigFileCreated={async () => {
        return false;
      }}
    />
  );
}

export function AgentConfiguredButNotStarted() {
  return <ShowState agentProcessState={{ status: 'not-started' }} />;
}

export function AgentStarting() {
  const abortControllerRef = useRef(new AbortController());

  useEffect(() => {
    return () => {
      abortControllerRef.current.abort();
    };
  }, []);

  const appContext = new MockAppContext({ appVersion: '17.0.0' });

  appContext.connectMyComputerService.downloadAgent = () =>
    new Promise((resolve, reject) => {
      abortControllerRef.current.signal.addEventListener('abort', () => reject);
    });

  return (
    <ShowState
      appContext={appContext}
      agentProcessState={{ status: 'not-started' }}
      autoStart={true}
    />
  );
}

export function AgentRunning() {
  return <ShowState agentProcessState={{ status: 'running' }} />;
}

export function AgentError() {
  return (
    <ShowState
      agentProcessState={{
        status: 'error',
        message: 'ENOENT file does not exist',
      }}
    />
  );
}

export function AgentExitedSuccessfully() {
  return (
    <ShowState
      agentProcessState={{
        status: 'exited',
        exitedSuccessfully: true,
        code: 0,
        signal: null,
      }}
    />
  );
}

export function AgentExitedUnsuccessfully() {
  return (
    <ShowState
      agentProcessState={{
        status: 'exited',
        exitedSuccessfully: false,
        code: 1,
        logs: 'teleport: error: unknown short flag -non-existing-flag',
        signal: null,
      }}
    />
  );
}

export function LoadingAgentConfigFile() {
  const abortControllerRef = useRef(new AbortController());

  useEffect(() => {
    return () => {
      abortControllerRef.current.abort();
    };
  }, []);

  const getPromiseRejectedOnUnmount = () =>
    new Promise<boolean>((resolve, reject) => {
      abortControllerRef.current.signal.addEventListener('abort', () => reject);
    });

  return (
    <ShowState
      agentProcessState={{ status: 'not-started' }}
      isAgentConfigFileCreated={getPromiseRejectedOnUnmount}
    />
  );
}

export function FailedToLoadAgentConfigFile() {
  return (
    <ShowState
      agentProcessState={{ status: 'not-started' }}
      isAgentConfigFileCreated={async () => {
        throw new Error('EPERM');
      }}
    />
  );
}

function ShowState({
  isAgentConfigFileCreated = async () => true,
  agentProcessState,
  appContext = new MockAppContext(),
  autoStart = false,
}: {
  agentProcessState: AgentProcessState;
  isAgentConfigFileCreated?: () => Promise<boolean>;
  appContext?: MockAppContext;
  autoStart?: boolean;
}) {
  const cluster = makeRootCluster({
    proxyVersion: '17.0.0',
  });
  appContext.clustersService.state.clusters.set(cluster.uri, cluster);
  appContext.workspacesService.setState(draftState => {
    draftState.rootClusterUri = cluster.uri;
    draftState.workspaces[cluster.uri] = {
      localClusterUri: cluster.uri,
      documents: [],
      location: undefined,
      accessRequests: undefined,
    };
  });

  appContext.mainProcessClient.getAgentState = () => agentProcessState;
  appContext.connectMyComputerService.isAgentConfigFileCreated =
    isAgentConfigFileCreated;

  if (autoStart) {
    appContext.workspacesService.setConnectMyComputerAutoStart(
      cluster.uri,
      true
    );
  }

  useLayoutEffect(() => {
    (
      document.querySelector(
        '[data-testid=connect-my-computer-icon]'
      ) as HTMLButtonElement
    )?.click();
  });

  return (
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider rootClusterUri={cluster.uri}>
        <ResourcesContextProvider>
          <ConnectMyComputerContextProvider rootClusterUri={cluster.uri}>
            <NavigationMenu />
          </ConnectMyComputerContextProvider>
        </ResourcesContextProvider>
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );
}
