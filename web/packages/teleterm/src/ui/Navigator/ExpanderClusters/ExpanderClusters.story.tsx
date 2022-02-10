/**
 * Copyright 2022 Gravitational, Inc.
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
import { ExpanderClustersPresentational } from 'teleterm/ui/Navigator/ExpanderClusters/ExpanderClusters';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { ExpanderClusterState } from './types';

export default {
  title: 'Teleterm/Navigator/ExpanderClusters',
};

function getState(): ExpanderClusterState {
  return {
    onAddCluster() {},
    onSyncClusters() {},
    onOpenContextMenu() {},
    onOpen() {},
    items: [
      {
        clusterUri: '/clusters/root1.teleport.sh',
        title: 'root1.teleport.sh',
        syncing: false,
        connected: false,
        active: false,
      },
      {
        clusterUri: '/clusters/root2.teleport.sh',
        title: 'root2.teleport.sh',
        syncing: false,
        connected: true,
        active: true,
      },
      {
        clusterUri: '/clusters/root3.teleport.sh',
        title: 'root3.teleport.sh',
        connected: true,
        syncing: false,
        active: false,
        leaves: [
          {
            clusterUri:
              '/clusters/root3.teleport.sh/leaves/internal1.example.com',
            title: 'internal1.example.com',
            syncing: false,
            connected: false,
            active: false,
          },
          {
            clusterUri:
              '/clusters/root3.teleport.sh/leaves/internal2.example.com',
            title: 'internal2.example.com',
            syncing: false,
            connected: true,
            active: false,
          },
        ],
      },
    ],
  };
}

export function Syncing() {
  const state = getState();
  state.items.forEach(i => {
    i.syncing = true;
  });
  return (
    <MockAppContextProvider>
      <ExpanderClustersPresentational {...state} />
    </MockAppContextProvider>
  );
}

export function ClusterItems() {
  const state = getState();
  return (
    <MockAppContextProvider>
      <ExpanderClustersPresentational {...state} />
    </MockAppContextProvider>
  );
}
