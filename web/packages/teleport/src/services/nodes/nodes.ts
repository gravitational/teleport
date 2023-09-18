/*
Copyright 2019-2022 Gravitational, Inc.

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

import api from 'teleport/services/api';
import cfg, { UrlResourcesParams } from 'teleport/config';
import { ResourcesResponse } from 'teleport/services/agents';

import { Node, CreateNodeRequest } from './types';
import makeNode from './makeNode';

class NodeService {
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

export default NodeService;
