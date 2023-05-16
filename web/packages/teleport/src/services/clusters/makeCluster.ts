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

import { displayDateTime } from 'shared/services/loc';

import cfg from 'teleport/config';

import { Cluster } from './types';

export function makeCluster(json): Cluster {
  const {
    name,
    lastConnected,
    status,
    nodeCount,
    publicURL,
    authVersion,
    proxyVersion,
  } = json;

  const lastConnectedDate = new Date(lastConnected);
  const connectedText = displayDateTime(lastConnectedDate);

  return {
    clusterId: name,
    lastConnected: lastConnectedDate,
    connectedText,
    status,
    url: cfg.getClusterRoute(name),
    authVersion,
    nodeCount,
    publicURL,
    proxyVersion,
  };
}

export function makeClusterList(json: any): Cluster[] {
  json = json || [];

  const clusters = json.map(cluster => makeCluster(cluster));

  // Sort by clusterId.
  return clusters.sort((a, b) => {
    if (a.clusterId < b.clusterId) {
      return -1;
    }
    if (a.clusterId > b.clusterId) {
      return 1;
    }
    return 0;
  });
}

export const StatusEnum = {
  OFFLINE: 'offline',
  ONLINE: 'online',
};
