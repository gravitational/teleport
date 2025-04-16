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

import { MemoryRouter } from 'react-router';

import { Box, Text } from 'design';

import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { Cluster } from 'teleport/services/clusters';

import { ClusterDropdown } from './ClusterDropdown';

export default {
  title: 'Shared/ClusterDropdown',
};

const fetchClusters = () => null;

export const Dropdown = () => {
  const ctx = createTeleportContext();
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <Box mb={4}>
          <Text>500 clusters</Text>
          <ClusterDropdown
            clusterId="cluster-2"
            clusterLoader={{
              fetchClusters,
              clusters: lotsOfClusters,
            }}
            onError={() => null}
          />
        </Box>
        <Box mb={4}>
          <Text>2 clusters</Text>
          <ClusterDropdown
            clusterId="cluster-2"
            clusterLoader={{
              fetchClusters,
              clusters: twoClusters,
            }}
            onError={() => null}
          />
        </Box>
        <Box mb={4}>
          <Text>20 clusters</Text>
          <ClusterDropdown
            clusterId="cluster-2"
            clusterLoader={{ fetchClusters, clusters: twentyClusters }}
            onError={() => null}
          />
        </Box>
        <Box mb={4}>
          <Text>5 clusters</Text>
          <ClusterDropdown
            clusterId="cluster-2"
            clusterLoader={{ fetchClusters, clusters: fiveClusters }}
            onError={() => null}
          />
        </Box>
        <Box mb={4}>
          <Text>no clusters (shouldn't be displayed)</Text>
          <ClusterDropdown
            clusterId="cluster-2"
            clusterLoader={{ clusters: [], fetchClusters }}
            onError={() => null}
          />
        </Box>
        <Box mb={4}>
          <Text>1 cluster (shouldn't be displayed)</Text>
          <ClusterDropdown
            clusterId="cluster-2"
            clusterLoader={{ clusters: oneCluster, fetchClusters }}
            onError={() => null}
          />
        </Box>
      </ContextProvider>
    </MemoryRouter>
  );
};

Dropdown.storyName = 'ClusterDropdown';

const lotsOfClusters = new Array(500).fill(null).map(
  (_, i) =>
    ({
      clusterId: `cluster-${i}`,
    }) as Cluster
);

const oneCluster = [
  {
    clusterId: `cluster-1`,
  } as Cluster,
];

const twoClusters = new Array(2).fill(null).map(
  (_, i) =>
    ({
      clusterId: `cluster-${i}`,
    }) as Cluster
);

const twentyClusters = new Array(20).fill(null).map(
  (_, i) =>
    ({
      clusterId: `cluster-${i}`,
    }) as Cluster
);

const fiveClusters = new Array(5).fill(null).map(
  (_, i) =>
    ({
      clusterId: `cluster-${i}`,
    }) as Cluster
);
