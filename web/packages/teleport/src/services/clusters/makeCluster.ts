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

import { displayDate, displayDateTime } from 'design/datetime';

import cfg from 'teleport/config';

import { Cluster, ClusterInfo } from './types';

export function makeCluster(json): Cluster {
  const {
    name,
    lastConnected,
    status,
    publicURL,
    authVersion,
    proxyVersion,
    licenseExpiry,
  } = json;

  const lastConnectedDate = new Date(lastConnected);
  const connectedText = displayDateTime(lastConnectedDate);

  let licenseExpiryDateText;
  if (licenseExpiry) {
    licenseExpiryDateText = displayDate(new Date(licenseExpiry));
  }

  return {
    clusterId: name,
    lastConnected: lastConnectedDate,
    connectedText,
    status,
    url: cfg.getClusterRoute(name),
    authVersion,
    publicURL,
    proxyVersion,
    licenseExpiryDateText,
  };
}

export function makeClusterInfo(json): ClusterInfo {
  const isCloud = json.isCloud;
  const cluster = makeCluster(json);
  return { ...cluster, isCloud };
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
