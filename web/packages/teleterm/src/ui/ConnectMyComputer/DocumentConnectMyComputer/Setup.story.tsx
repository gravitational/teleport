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

import {
  makeRootCluster,
  makeServer,
} from 'teleterm/services/tshd/testHelpers';
import { Cluster, LoggedInUser_UserType } from 'teleterm/services/tshd/types';
import { ResourcesContextProvider } from 'teleterm/ui/DocumentCluster/resourcesContext';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';

import { ConnectMyComputerContextProvider } from '../connectMyComputerContext';
import { Setup } from './Setup';

export default {
  title: 'Teleterm/ConnectMyComputer/Setup',
};

export function Default() {
  const cluster = makeRootCluster();
  const appContext = new MockAppContext({ appVersion: cluster.proxyVersion });
  appContext.connectMyComputerService.waitForNodeToJoin = async () =>
    makeServer();
  return (
    <ShowState
      cluster={cluster}
      appContext={appContext}
      clickStartSetup={false}
    />
  );
}

export function Success() {
  const cluster = makeRootCluster();
  const appContext = new MockAppContext({ appVersion: cluster.proxyVersion });
  appContext.connectMyComputerService.waitForNodeToJoin = async () =>
    makeServer();
  // Report the agent as running so that the autostart behavior doesn't kick in and attempt to start
  // the agent over and over.
  appContext.mainProcessClient.subscribeToAgentUpdate = (
    rootClusterUri,
    callback
  ) => {
    callback({ status: 'running' });

    return { cleanup: () => {} };
  };
  return <ShowState cluster={cluster} appContext={appContext} />;
}

export function Errored() {
  const cluster = makeRootCluster();
  const appContext = new MockAppContext({ appVersion: cluster.proxyVersion });
  appContext.connectMyComputerService.createAgentConfigFile = () => {
    throw new Error('Failed to write file, no permissions.');
  };
  return <ShowState cluster={cluster} appContext={appContext} />;
}

export function InProgress() {
  const cluster = makeRootCluster();
  const appContext = new MockAppContext({ appVersion: cluster.proxyVersion });
  const ref = useRef(new AbortController());

  useEffect(() => {
    return () => ref.current.abort();
  }, []);

  appContext.connectMyComputerService.createRole = () =>
    new Promise(resolve => {
      ref.current.signal.addEventListener('abort', () => resolve(undefined));
    });

  return <ShowState cluster={cluster} appContext={appContext} />;
}

export function AgentVersionTooNew() {
  const cluster = makeRootCluster({ proxyVersion: '16.3.0' });
  const appContext = new MockAppContext({ appVersion: '17.0.0' });

  return (
    <ShowState
      cluster={cluster}
      appContext={appContext}
      clickStartSetup={false}
    />
  );
}

export function AgentVersionTooOld() {
  const cluster = makeRootCluster({ proxyVersion: '16.3.0' });
  const appContext = new MockAppContext({ appVersion: '14.1.0' });
  return (
    <ShowState
      cluster={cluster}
      appContext={appContext}
      clickStartSetup={false}
    />
  );
}

export function NoAccess() {
  const cluster = makeRootCluster();
  cluster.loggedInUser.acl.tokens.create = false;
  const appContext = new MockAppContext({});

  return (
    <ShowState
      cluster={cluster}
      appContext={appContext}
      clickStartSetup={false}
    />
  );
}

export function AccessUnknown() {
  const cluster = makeRootCluster();
  cluster.loggedInUser.userType = LoggedInUser_UserType.UNSPECIFIED;
  const appContext = new MockAppContext({});

  return (
    <ShowState
      cluster={cluster}
      appContext={appContext}
      clickStartSetup={false}
    />
  );
}

function ShowState({
  cluster,
  appContext,
  clickStartSetup = true,
}: {
  cluster: Cluster;
  appContext: MockAppContext;
  clickStartSetup?: boolean;
}) {
  if (!appContext.clustersService.state.clusters.get(cluster.uri)) {
    appContext.addRootCluster(cluster);
  }

  useLayoutEffect(() => {
    if (clickStartSetup) {
      (
        document.querySelector('[data-testid=start-setup]') as HTMLButtonElement
      )?.click();
    }
  }, [clickStartSetup]);

  return (
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider rootClusterUri={cluster.uri}>
        <ResourcesContextProvider>
          <ConnectMyComputerContextProvider rootClusterUri={cluster.uri}>
            <Setup updateDocumentStatus={() => {}} />
          </ConnectMyComputerContextProvider>
        </ResourcesContextProvider>
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );
}
