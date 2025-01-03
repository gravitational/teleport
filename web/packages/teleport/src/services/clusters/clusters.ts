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

import cfg from 'teleport/config';
import api from 'teleport/services/api';

import { Cluster } from '.';
import { makeClusterInfo, makeClusterList } from './makeCluster';

export default class ClustersService {
  clusters: Cluster[] = [];

  fetchClusters(abortSignal?: AbortSignal, fromCache = true) {
    if (!fromCache) {
      return api.get(cfg.api.clustersPath, abortSignal).then(makeClusterList);
    }
    if (this.clusters.length < 1) {
      return api.get(cfg.api.clustersPath, abortSignal).then(res => {
        // cache the result of clusters response
        this.clusters = makeClusterList(res);
        return this.clusters;
      });
    }
    return Promise.resolve(this.clusters);
  }

  fetchClusterDetails(clusterId) {
    return api.get(cfg.getClusterInfoPath(clusterId)).then(makeClusterInfo);
  }
}
