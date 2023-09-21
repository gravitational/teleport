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

import React from 'react';

import { wait } from 'shared/utils/wait';

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import AppContext from 'teleterm/ui/appContext';

import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { AgentProcessState } from 'teleterm/mainProcess/types';
import { makeRuntimeSettings } from 'teleterm/mainProcess/fixtures/mocks';

import { ConnectMyComputerContextProvider } from '../connectMyComputerContext';

import { DocumentConnectMyComputerStatus } from './DocumentConnectMyComputerStatus';

export default {
  title: 'Teleterm/ConnectMyComputer/Status',
};

export function NotStarted() {
  return <ShowState agentProcessState={{ status: 'not-started' }} />;
}

export function Running() {
  return <ShowState agentProcessState={{ status: 'running' }} />;
}

export function Errored() {
  return (
    <ShowState
      agentProcessState={{
        status: 'error',
        message: 'ENOENT file does not exist',
      }}
    />
  );
}

export function ExitedSuccessfully() {
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

export function ExitedUnsuccessfully() {
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

export function FailedToReadAgentConfigFile() {
  const appContext = new MockAppContext();
  appContext.connectMyComputerService.isAgentConfigFileCreated = async () => {
    throw new Error('EPERM');
  };

  return (
    <ShowState
      agentProcessState={{ status: 'not-started' }}
      appContext={appContext}
    />
  );
}

export function AgentVersionTooNew() {
  const appContext = new MockAppContext({ appVersion: '17.0.0' });

  return (
    <ShowState
      agentProcessState={{ status: 'not-started' }}
      appContext={appContext}
      proxyVersion={'16.3.0'}
    />
  );
}

// Shows only cluster upgrade instructions.
// Downgrading the app would result in installing a version that doesn't support 'Connect My Computer'.
// DELETE IN 17.0.0 (gzdunek): by the time 17.0 releases, 14.x will no longer be
// supported, so downgrade will be always possible.
export function AgentVersionTooNewButOnlyClusterCanBeUpgraded() {
  const appContext = new MockAppContext({ appVersion: '14.1.0' });

  return (
    <ShowState
      agentProcessState={{ status: 'not-started' }}
      appContext={appContext}
      proxyVersion={'13.3.0'}
    />
  );
}

export function AgentVersionTooOld() {
  const appContext = new MockAppContext({ appVersion: '14.1.0' });

  return (
    <ShowState
      agentProcessState={{ status: 'not-started' }}
      appContext={appContext}
      proxyVersion={'16.3.0'}
    />
  );
}

export function UpgradeAgentSuggestion() {
  const appContext = new MockAppContext({ appVersion: '15.2.0' });

  return (
    <ShowState
      agentProcessState={{ status: 'not-started' }}
      appContext={appContext}
      proxyVersion={'16.3.0'}
    />
  );
}

function ShowState(props: {
  agentProcessState: AgentProcessState;
  appContext?: AppContext;
  proxyVersion?: string;
}) {
  const cluster = makeRootCluster({
    proxyVersion: props.proxyVersion || makeRuntimeSettings().appVersion,
  });
  const appContext =
    props.appContext ||
    new MockAppContext({ appVersion: cluster.proxyVersion });

  appContext.mainProcessClient.getAgentState = () => props.agentProcessState;
  appContext.mainProcessClient.subscribeToAgentUpdate = (
    rootClusterUri,
    listener
  ) => {
    listener(props.agentProcessState);
    return { cleanup: () => undefined };
  };
  appContext.connectMyComputerService.runAgent = async () => {
    await wait(1_000);
  };
  appContext.connectMyComputerService.waitForNodeToJoin = async () => {
    if (props.agentProcessState.status === 'running') {
      await wait(2_000);
      return {
        uri: `${cluster.uri}/servers/178ef081-259b-4aa5-a018-449b5ea7e694`,
        tunnel: false,
        name: '178ef081-259b-4aa5-a018-449b5ea7e694',
        hostname: 'staging-mac-mini',
        addr: '127.0.0.1:3022',
        labelsList: [
          {
            name: 'hostname',
            value: 'staging-mac-mini',
          },
          {
            name: 'teleport.dev/connect-my-computer',
            value: 'testuser@goteleport.com',
          },
        ],
      };
    }
    await wait(3_000);
    throw new Error('TIMEOUT. Cannot find node.');
  };
  appContext.clustersService.state.clusters.set(cluster.uri, cluster);
  appContext.workspacesService.setState(draftState => {
    draftState.rootClusterUri = cluster.uri;
  });

  return (
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider rootClusterUri={cluster.uri}>
        <ConnectMyComputerContextProvider rootClusterUri={cluster.uri}>
          <DocumentConnectMyComputerStatus />
        </ConnectMyComputerContextProvider>
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );
}
