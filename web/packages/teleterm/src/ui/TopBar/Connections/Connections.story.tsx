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
          targetName: 'graves',
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
    async removeItem() {},
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
