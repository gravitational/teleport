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

import cfg, {
  UrlKubeResourcesParams,
  UrlResourcesParams,
} from 'teleport/config';
import { ResourcesResponse } from 'teleport/services/agents';
import api from 'teleport/services/api';

import { makeKube, makeKubeResource } from './makeKube';
import { Kube, KubeResourceResponse } from './types';

class KubeService {
  fetchKubernetes(
    clusterId,
    params: UrlResourcesParams,
    signal?: AbortSignal
  ): Promise<ResourcesResponse<Kube>> {
    return api
      .get(cfg.getKubernetesUrl(clusterId, params), signal)
      .then(json => {
        const items = json?.items || [];

        return {
          agents: items.map(makeKube),
          startKey: json?.startKey,
          totalCount: json?.totalCount,
        };
      });
  }

  fetchKubernetesResources(
    clusterId,
    params: UrlKubeResourcesParams,
    signal?: AbortSignal
  ): Promise<KubeResourceResponse> {
    return api
      .get(cfg.getKubernetesResourcesUrl(clusterId, params), signal)
      .then(json => {
        const items = json?.items || [];

        return {
          items: items.map(makeKubeResource),
          startKey: json?.startKey,
          totalCount: json?.totalCount,
        };
      });
  }
}

export default KubeService;
