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

import { Meta } from '@storybook/react';
import { useLayoutEffect } from 'react';

import { Flex, Text } from 'design';
import { CheckReportStatus } from 'gen-proto-ts/teleport/lib/vnet/diag/v1/diag_pb';

import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import {
  makeCheckAttempt,
  makeCheckReport,
  makeReport,
} from 'teleterm/services/vnet/testHelpers';
import AppContextProvider from 'teleterm/ui/appContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { VnetContextProvider } from 'teleterm/ui/Vnet';

import { Connections } from './Connections';
import { ConnectionsContextProvider } from './connectionsContext';

const meta: Meta = {
  title: 'Teleterm/TopBar/Connections',
  decorators: [
    Story => {
      useOpenConnections();

      return <Story />;
    },
  ],
};
export default meta;

const rootClusterUri = '/clusters/foo';

export function MultipleClusters() {
  const appContext = new MockAppContext();
  prepareAppContext(appContext);
  appContext.clustersService.setState(draft => {
    const rootCluster1 = makeRootCluster({
      uri: rootClusterUri,
      name: 'teleport.example.sh',
    });
    const rootCluster2 = makeRootCluster({
      uri: '/clusters/bar',
      name: 'bar.example.com',
    });
    draft.clusters.set(rootCluster1.uri, rootCluster1);
    draft.clusters.set(rootCluster2.uri, rootCluster2);
  });
  appContext.connectionTracker.getConnections = () => [
    ...makeConnections(),
    {
      connected: true,
      kind: 'connection.server' as const,
      title: 'runner-prod',
      id: 'ed23ded1',
      serverUri: '/clusters/bar/servers/ed23ded1',
      login: 'alice',
      clusterName: 'bar.example.com',
    },
  ];

  return (
    <AppContextProvider value={appContext}>
      <ConnectionsContextProvider>
        <VnetContextProvider>
          <Connections />
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </AppContextProvider>
  );
}

export function SingleCluster() {
  const appContext = new MockAppContext();
  prepareAppContext(appContext);
  appContext.clustersService.setState(draft => {
    const rootCluster1 = makeRootCluster({
      uri: rootClusterUri,
      name: 'teleport.example.sh',
      proxyHost: 'teleport.example.sh:443',
    });
    draft.clusters.set(rootCluster1.uri, rootCluster1);
  });

  return (
    <AppContextProvider value={appContext}>
      <ConnectionsContextProvider>
        <VnetContextProvider>
          <Connections />
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </AppContextProvider>
  );
}

export function JustVnetWithNoClusters() {
  const appContext = new MockAppContext();
  prepareAppContext(appContext);
  appContext.connectionTracker.getConnections = () => [];

  return (
    <AppContextProvider value={appContext}>
      <ConnectionsContextProvider>
        <VnetContextProvider>
          <Connections />
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </AppContextProvider>
  );
}

export function VnetWarning() {
  const appContext = new MockAppContext();
  prepareAppContext(appContext);

  appContext.configService.set('unstable.vnetDiag', true);
  appContext.statePersistenceService.putState({
    ...appContext.statePersistenceService.getState(),
    vnet: { autoStart: true },
  });
  appContext.workspacesService.setState(draft => {
    draft.isInitialized = true;
  });
  appContext.vnet.runDiagnostics = () =>
    new MockedUnaryCall({
      report: makeReport({
        checks: [
          makeCheckAttempt({
            checkReport: makeCheckReport({
              status: CheckReportStatus.ISSUES_FOUND,
            }),
          }),
        ],
      }),
    });

  return (
    <AppContextProvider value={appContext}>
      <ConnectionsContextProvider>
        <VnetContextProvider>
          <Connections />
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </AppContextProvider>
  );
}

export function VnetError() {
  const appContext = new MockAppContext();
  prepareAppContext(appContext);

  appContext.statePersistenceService.putState({
    ...appContext.statePersistenceService.getState(),
    vnet: { autoStart: true },
  });
  appContext.workspacesService.setState(draft => {
    draft.isInitialized = true;
  });
  appContext.vnet.start = () =>
    new MockedUnaryCall({}, new Error('something went wrong'));

  return (
    <AppContextProvider value={appContext}>
      <ConnectionsContextProvider>
        <VnetContextProvider>
          <Connections />
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </AppContextProvider>
  );
}

export function WithScroll() {
  const appContext = new MockAppContext();
  prepareAppContext(appContext);
  appContext.connectionTracker.getConnections = () => [
    {
      connected: false,
      kind: 'connection.server' as const,
      title: 'last-item',
      id: 'last-item',
      serverUri: '/clusters/foo/servers/last-item',
      login: 'item',
      clusterName: 'teleport.example.sh',
    },
    ...Array(10)
      .fill(undefined)
      .flatMap((_, index) => makeConnections(index)),
  ];

  return (
    <Flex
      flexDirection="row"
      justifyContent="space-between"
      gap={3}
      minWidth="500px"
      maxWidth="600px"
    >
      <AppContextProvider value={appContext}>
        <ConnectionsContextProvider>
          <VnetContextProvider>
            <Connections />
          </VnetContextProvider>
        </ConnectionsContextProvider>
      </AppContextProvider>
      <Text
        css={`
          max-width: 20ch;
        `}
      >
        Manipulate window height to simulate how the list behaves in Connect.
      </Text>
    </Flex>
  );
}

export function WithoutVnet() {
  const appContext = new MockAppContext({ platform: 'win32' });
  prepareAppContext(appContext);

  return (
    <AppContextProvider value={appContext}>
      <ConnectionsContextProvider>
        <VnetContextProvider>
          <Connections />
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </AppContextProvider>
  );
}

export function EmptyWithoutVnet() {
  const appContext = new MockAppContext({ platform: 'win32' });

  return (
    <AppContextProvider value={appContext}>
      <ConnectionsContextProvider>
        <VnetContextProvider>
          <Connections />
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </AppContextProvider>
  );
}

const makeConnections = (index = 0) => {
  const suffix = index === 0 ? '' : `-${index}`;

  return [
    {
      connected: true,
      kind: 'connection.server' as const,
      title: 'ansible' + suffix,
      id: 'e9c4fbc2' + suffix,
      serverUri: '/clusters/foo/servers/ansible' + suffix,
      login: 'casey',
      clusterName: 'teleport.example.sh',
    },
    {
      connected: true,
      kind: 'connection.gateway' as const,
      title: 'postgres' + suffix,
      targetName: 'postgres',
      id: '68b6a281' + suffix,
      targetUri: '/clusters/foo/dbs/brock' + suffix,
      port: '22',
      gatewayUri: '/gateways/empty',
      clusterName: 'teleport.example.sh',
    },
    {
      connected: false,
      kind: 'connection.server' as const,
      title: 'ansible-staging' + suffix,
      id: '949651ed' + suffix,
      serverUri: '/clusters/foo/servers/ansible-staging' + suffix,
      login: 'casey',
      clusterName: 'teleport.example.sh',
    },
  ];
};

const prepareAppContext = (appContext: MockAppContext) => {
  appContext.connectionTracker.getConnections = () => makeConnections();
  appContext.connectionTracker.activateItem = async () => {};
  appContext.connectionTracker.disconnectItem = async () => {};
  appContext.connectionTracker.removeItem = async () => {};
  appContext.connectionTracker.useState = () => null;
};

const useOpenConnections = () => {
  useLayoutEffect(() => {
    const areConnectionsOpen = !!document.querySelector(
      'input[role=searchbox]'
    );

    if (areConnectionsOpen) {
      return;
    }

    const button = document.querySelector(
      'button[title~="connections"i]'
    ) as HTMLButtonElement;

    button?.click();
  }, []);
};
