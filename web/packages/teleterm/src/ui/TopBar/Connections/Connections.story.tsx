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

import { useLayoutEffect } from 'react';
import { Flex, Text } from 'design';

import AppContextProvider from 'teleterm/ui/appContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { ExtendedTrackedConnection } from 'teleterm/ui/services/connectionTracker';

import { Connections } from './Connections';

export default {
  title: 'Teleterm/TopBar/Connections',
  decorators: [
    Story => {
      useOpenConnections();

      return <Story />;
    },
  ],
};

export function Story() {
  const appContext = new MockAppContext();
  prepareAppContext(appContext);

  return (
    <AppContextProvider value={appContext}>
      <Connections />
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
        <Connections />
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

export function Empty() {
  const appContext = new MockAppContext({ platform: 'win32' });

  return (
    <AppContextProvider value={appContext}>
      <Connections />
    </AppContextProvider>
  );
}

const makeConnections = (index = 0): ExtendedTrackedConnection[] => {
  const suffix = index === 0 ? '' : `-${index}`;

  return [
    {
      connected: true,
      kind: 'connection.server' as const,
      title: 'ansible' + suffix,
      id: 'e9c4fbc2' + suffix,
      serverUri: `/clusters/foo/servers/ansible${suffix}`,
      login: 'casey',
      clusterName: 'teleport.example.sh',
    },
    {
      connected: true,
      kind: 'connection.gateway' as const,
      title: 'postgres' + suffix,
      targetName: 'postgres',
      id: '68b6a281' + suffix,
      targetUri: `/clusters/foo/dbs/brock${suffix}`,
      port: '22',
      gatewayUri: '/gateways/empty',
      clusterName: 'teleport.example.sh',
    },
    {
      connected: false,
      kind: 'connection.server' as const,
      title: 'ansible-staging' + suffix,
      id: '949651ed' + suffix,
      serverUri: `/clusters/foo/servers/ansible-staging${suffix}`,
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
  });
};
