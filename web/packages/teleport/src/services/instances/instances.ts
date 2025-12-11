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

import cfg from 'teleport/config';
import type { SortType } from 'teleport/services/agents';
import api from 'teleport/services/api';

import type { UnifiedInstancesResponse } from './types';

export async function fetchInstances(
  variables: {
    clusterId: string;
    limit: number;
    startKey?: string;
    query?: string;
    search?: string;
    sort?: SortType;
    types?: string;
    services?: string;
    upgraders?: string;
  },
  signal?: AbortSignal
): Promise<UnifiedInstancesResponse> {
  const { clusterId, ...params } = variables;

  const response = await api.get(
    cfg.getInstancesUrl(clusterId, params),
    signal
  );

  return {
    instances: response?.instances || [],
    startKey: response?.startKey,
  };
}
