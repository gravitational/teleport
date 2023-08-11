/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { useRouteMatch } from 'react-router';
import { useRef } from 'react';

import { StickyCluster } from 'teleport/types';
import cfg from 'teleport/config';

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
