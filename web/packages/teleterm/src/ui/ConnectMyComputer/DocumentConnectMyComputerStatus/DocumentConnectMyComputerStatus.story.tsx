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

const cluster = makeRootCluster();

function ShowState(props: {
  agentProcessState: AgentProcessState;
  appContext?: AppContext;
}) {
  const appContext = props.appContext || new MockAppContext();

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
