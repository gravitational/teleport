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
        stackTrace: 'teleport: error: unknown short flag -non-existing-flag',
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
  const doc: types.DocumentConnectMyComputerSetup = {
    kind: 'doc.connect_my_computer_setup',
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
