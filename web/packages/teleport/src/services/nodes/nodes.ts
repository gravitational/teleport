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

import api from 'teleport/services/api';
import cfg from 'teleport/config';
import makeNode from './makeNode';
import makeNodeToken from './makeNodeToken';
import { NodeToken, NodesResponse } from './types';

const service = {
  fetchNodes(clusterId?: string): Promise<NodesResponse> {
    return api.get(cfg.getClusterNodesUrl(clusterId)).then(json => {
      const items = json?.items || [];

      return {
        nodes: items.map(makeNode),
        startKey: json?.startKey,
        totalCount: json?.totalCount,
      };
    });
  },

  fetchJoinToken(): Promise<NodeToken> {
    return api.post(cfg.getNodeJoinTokenUrl()).then(makeNodeToken);
  },
};

export default service;
