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

import cfg, { UrlResourcesParams } from 'teleport/config';
import { ResourcesResponse } from 'teleport/services/agents';
import api from 'teleport/services/api';

import makeNode from './makeNode';
import { CreateNodeRequest, Node } from './types';

export default class NodeService {
  fetchNodes(
    clusterId?: string,
    params?: UrlResourcesParams,
    signal?: AbortSignal
  ): Promise<ResourcesResponse<Node>> {
    return api
      .get(cfg.getClusterNodesUrl(clusterId, params), signal)
      .then(json => {
        const items = json?.items || [];

        return {
          agents: items.map(makeNode),
          startKey: json?.startKey,
          totalCount: json?.totalCount,
        };
      });
  }

  // Creates a Node.
  createNode(clusterId: string, req: CreateNodeRequest): Promise<Node> {
    return api
      .post(cfg.getClusterNodesUrlNoParams(clusterId), req)
      .then(makeNode);
  }
}

/**
 * Sorts logins alphabetically. If the logins include "root", it's put as the first item in the
 * resulting list.
 */
export const sortNodeLogins = (logins: string[]) => {
  const noRoot = logins.filter(l => l !== 'root').sort();
  if (noRoot.length === logins.length) {
    return logins;
  }
  return ['root', ...noRoot];
};
