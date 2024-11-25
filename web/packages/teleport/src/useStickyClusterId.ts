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

import { useRef } from 'react';
import { useRouteMatch } from 'react-router';

import cfg from 'teleport/config';
import { StickyCluster } from 'teleport/types';

// useStickyClusterId determines the current :clusterId in the URL.
// When a route contains `:clusterId` it can refer to the root or the leaf cluster.
export default function useStickyClusterId(): StickyCluster {
  // assign initial values where the default cluster is a proxy
  const stickyCluster = useRef({
    clusterId: cfg.proxyCluster,
    hasClusterUrl: false,
    isLeafCluster: false,
  });

  const match = useRouteMatch<{ clusterId: string }>(cfg.routes.cluster);
  const clusterId = match?.params?.clusterId;
  if (clusterId) {
    stickyCluster.current.clusterId = clusterId;
    stickyCluster.current.isLeafCluster = clusterId !== cfg.proxyCluster;
  }

  stickyCluster.current.hasClusterUrl = !!clusterId;

  return stickyCluster.current;
}
