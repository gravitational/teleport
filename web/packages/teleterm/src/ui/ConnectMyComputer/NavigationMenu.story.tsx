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

import React from 'react';

import { Flex } from 'design';

import { wait } from 'shared/utils/wait';

import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import * as types from 'teleterm/ui/services/workspacesService';
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
  return (
    <ShowState
      agentProcessState={{ status: 'not-started' }}
      isAgentConfigFileCreated={async () => {
        await wait(60_000);
        return true;
      }}
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
}: {
  agentProcessState: AgentProcessState;
  isAgentConfigFileCreated?: () => Promise<boolean>;
}) {
  const cluster = makeRootCluster({
    features: { isUsageBasedBilling: true, advancedAccessWorkflows: false },
  });
  cluster.loggedInUser.acl.tokens = {
    create: true,
    use: true,
    read: true,
    list: true,
    edit: true,
    pb_delete: true,
  };
  const doc: types.DocumentConnectMyComputer = {
    kind: 'doc.connect_my_computer',
    rootClusterUri: cluster.uri,
    title: 'Connect My Computer',
    uri: '/docs/123',
  };
  const appContext = new MockAppContext();
  appContext.clustersService.state.clusters.set(cluster.uri, cluster);
  appContext.configService = createMockConfigService({
    'feature.connectMyComputer': true,
  });
  appContext.workspacesService.setState(draftState => {
    draftState.rootClusterUri = cluster.uri;
    draftState.workspaces[cluster.uri] = {
      localClusterUri: cluster.uri,
      documents: [doc],
      location: doc.uri,
      accessRequests: undefined,
    };
  });

  appContext.mainProcessClient.getAgentState = () => agentProcessState;
  appContext.connectMyComputerService.isAgentConfigFileCreated =
    isAgentConfigFileCreated;

  return (
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider rootClusterUri={cluster.uri}>
        <ConnectMyComputerContextProvider rootClusterUri={cluster.uri}>
          <Flex justifyContent="flex-end">
            <NavigationMenu clusterUri={cluster.uri} />
          </Flex>
        </ConnectMyComputerContextProvider>
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );
}
