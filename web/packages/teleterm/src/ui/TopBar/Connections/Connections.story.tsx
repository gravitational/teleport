import React from 'react';

import AppContextProvider from 'teleterm/ui/appContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';

import { Connections } from './Connections';

export default {
  title: 'Teleterm/TopBar/Connections',
};

export function ExpanderConnections() {
  const appContext = new MockAppContext();
  appContext.connectionTracker.getConnections = () => {
    return [
      {
        connected: true,
        kind: 'connection.server',
        title: 'ansible',
        id: 'e9c4fbc2',
        serverUri: '/clusters/foo/servers/ansible',
        login: 'casey',
        clusterName: 'teleport.example.sh',
      },
      {
        connected: true,
        kind: 'connection.gateway',
        title: 'postgres',
        targetName: 'postgres',
        id: '68b6a281',
        targetUri: '/clusters/foo/dbs/brock',
        port: '22',
        gatewayUri: '/gateways/empty',
        clusterName: 'teleport.example.sh',
      },
      {
        connected: false,
        kind: 'connection.server',
        title: 'ansible-staging',
        id: '949651ed',
        serverUri: '/clusters/foo/servers/ansible-staging',
        login: 'casey',
        clusterName: 'teleport.example.sh',
      },
    ];
  };
  appContext.connectionTracker.activateItem = async () => {};
  appContext.connectionTracker.disconnectItem = async () => {};
  appContext.connectionTracker.removeItem = async () => {};
  appContext.connectionTracker.useState = () => null;

  return (
    <AppContextProvider value={appContext}>
      <Connections />
    </AppContextProvider>
  );
}
