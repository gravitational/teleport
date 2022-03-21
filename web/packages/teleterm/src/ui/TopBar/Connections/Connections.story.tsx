import React from 'react';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { ConnectionTrackerService } from 'teleterm/ui/services/connectionTracker';
import { Connections } from './Connections';

export default {
  title: 'Teleterm/TopBar/Connections',
};

export function ExpanderConnections() {
  const connectionTracker: Partial<ConnectionTrackerService> = {
    getConnections() {
      return [
        {
          connected: true,
          kind: 'connection.server',
          title: 'graves',
          id: 'e9c4fbc2',
          serverUri: 'brock',
          login: 'casey',
          clusterName: 'teleport.example.sh',
        },
        {
          connected: true,
          kind: 'connection.gateway',
          title: 'graves',
          id: '68b6a281',
          targetUri: 'brock',
          port: '22',
          gatewayUri: 'empty',
          clusterName: 'teleport.example.sh',
        },
        {
          connected: false,
          kind: 'connection.server',
          title: 'graves',
          id: '949651ed',
          serverUri: 'brock',
          login: 'casey',
          clusterName: 'teleport.example.sh',
        },
      ];
    },
    async activateItem() {},
    async disconnectItem() {},
    removeItem() {},
    useState() {
      return null;
    },
  };

  return (
    // @ts-expect-error - using mocks
    <MockAppContextProvider appContext={{ connectionTracker }}>
      <Connections />
    </MockAppContextProvider>
  );
}
