/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import cfg, { UrlResourcesParams } from 'teleport/config';
import api from 'teleport/services/api';

import type { UnifiedInstancesResponse } from './types';

export interface InstancesParams extends UrlResourcesParams {
  /** Filter to return instances, bot instances, or both */
  type?: 'instance' | 'bot_instance';
  /** Filter by service types (comma-separated) */
  services?: string;
  /** Filter by external upgrader types (comma-separated) */
  upgraders?: string;
  /** Sort order (asc or desc) */
  order?: 'asc' | 'desc';
}

class InstancesService {
  fetchInstances(
    clusterId: string,
    params?: InstancesParams,
    signal?: AbortSignal
  ): Promise<UnifiedInstancesResponse> {
    return api
      .get(cfg.getInstancesUrl(clusterId, params), signal)
      .then(json => {
        const instances = json?.instances || [];

        return {
          instances,
          startKey: json?.startKey,
        };
      });
  }
}

export default InstancesService;
