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

import React, { useEffect, useRef, useLayoutEffect } from 'react';

import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';

import { createMockConfigService } from 'teleterm/services/config/fixtures/mocks';

import { AgentProcessState } from 'teleterm/mainProcess/types';

import { NavigationMenu } from './NavigationMenu';
import { ConnectMyComputerContextProvider } from './connectMyComputerContext';

export default {
  title: 'Teleterm/ConnectMyComputer/NavigationMenu',
};

export function AgentRunning() {
  return <ShowState agentProcessState={{ status: 'running' }} />;
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

export function AgentSetupNotDone() {
  return (
    <ShowState
      agentProcessState={{ status: 'not-started' }}
      isAgentConfigFileCreated={async () => {
        return false;
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
    features: { isUsageBasedBilling: true, advancedAccessWorkflows: false },
    proxyVersion: '17.0.0',
  });
  appContext.clustersService.state.clusters.set(cluster.uri, cluster);
  appContext.configService = createMockConfigService({
    'feature.connectMyComputer': true,
  });
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
        <ConnectMyComputerContextProvider rootClusterUri={cluster.uri}>
          <NavigationMenu />
        </ConnectMyComputerContextProvider>
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );
}
