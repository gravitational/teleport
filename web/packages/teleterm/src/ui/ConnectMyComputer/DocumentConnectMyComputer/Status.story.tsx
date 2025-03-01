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

import { useLayoutEffect } from 'react';

import { makeRuntimeSettings } from 'teleterm/mainProcess/fixtures/mocks';
import { AgentProcessState } from 'teleterm/mainProcess/types';
import {
  makeLabelsList,
  makeRootCluster,
  makeServer,
} from 'teleterm/services/tshd/testHelpers';
import { ResourcesContextProvider } from 'teleterm/ui/DocumentCluster/resourcesContext';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';

import {
  AgentCompatibilityError,
  ConnectMyComputerContextProvider,
  NodeWaitJoinTimeout,
} from '../connectMyComputerContext';
import { Status } from './Status';

export default {
  title: 'Teleterm/ConnectMyComputer/Status',
};

export function NotStarted() {
  return <ShowState agentProcessState={{ status: 'not-started' }} />;
}

export function Running() {
  const appContext = new MockAppContext({ appVersion: '17.0.0' });

  let agentUpdateListener: (state: AgentProcessState) => void;
  appContext.mainProcessClient.subscribeToAgentUpdate = (
    rootClusterUri,
    listener
  ) => {
    agentUpdateListener = listener;
    return { cleanup: () => undefined };
  };
  appContext.connectMyComputerService.isAgentConfigFileCreated = () =>
    Promise.resolve(true);
  appContext.connectMyComputerService.runAgent = async () => {
    agentUpdateListener({ status: 'running' });
  };
  appContext.connectMyComputerService.waitForNodeToJoin = () =>
    Promise.resolve(
      makeServer({
        hostname: 'staging-mac-mini',
        labels: makeLabelsList({
          hostname: 'staging-mac-mini',
          'teleport.dev/connect-my-computer/owner': 'testuser@goteleport.com',
        }),
      })
    );

  useLayoutEffect(() => {
    (
      document.querySelector('[data-testid=start-agent]') as HTMLButtonElement
    )?.click();
  });

  return (
    <ShowState
      appContext={appContext}
      agentProcessState={{ status: 'not-started' }}
      proxyVersion="17.0.0"
    />
  );
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

export function ErrorWithAlertAndLogs() {
  const appContext = new MockAppContext({ appVersion: '17.0.0' });

  appContext.connectMyComputerService.isAgentConfigFileCreated = () =>
    Promise.resolve(true);
  appContext.connectMyComputerService.waitForNodeToJoin = () =>
    Promise.reject(
      new NodeWaitJoinTimeout(
        'teleport: error: unknown short flag -non-existing-flag'
      )
    );

  return (
    <ShowState
      appContext={appContext}
      agentProcessState={{
        status: 'not-started',
      }}
      autoStart={true}
      proxyVersion="17.0.0"
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

export function AgentVersionTooNewWithFailedAutoStart() {
  const appContext = new MockAppContext({ appVersion: '17.0.0' });

  appContext.connectMyComputerService.downloadAgent = () =>
    Promise.reject(new AgentCompatibilityError('incompatible'));
  appContext.connectMyComputerService.isAgentConfigFileCreated = () =>
    Promise.resolve(true);

  return (
    <ShowState
      agentProcessState={{ status: 'not-started' }}
      appContext={appContext}
      proxyVersion={'16.3.0'}
      autoStart={true}
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
  appContext?: MockAppContext;
  proxyVersion?: string;
  autoStart?: boolean;
}) {
  const cluster = makeRootCluster({
    proxyVersion: props.proxyVersion || makeRuntimeSettings().appVersion,
  });
  const appContext =
    props.appContext ||
    new MockAppContext({ appVersion: cluster.proxyVersion });

  appContext.mainProcessClient.getAgentState = () => props.agentProcessState;
  appContext.addRootCluster(cluster);

  if (props.autoStart) {
    appContext.workspacesService.setConnectMyComputerAutoStart(
      cluster.uri,
      true
    );
  }

  return (
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider rootClusterUri={cluster.uri}>
        <ResourcesContextProvider>
          <ConnectMyComputerContextProvider rootClusterUri={cluster.uri}>
            <Status />
          </ConnectMyComputerContextProvider>
        </ResourcesContextProvider>
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );
}
