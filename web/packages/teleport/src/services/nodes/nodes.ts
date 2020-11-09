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

import { map } from 'lodash';
import api from 'teleport/services/api';
import cfg from 'teleport/config';
import makeNode from './makeNode';
import makeNodeToken from './makeNodeToken';
import makeAppBashCmd from './makeAppBashCmd';
import makeNodeBashCmd from './makeNodeBashCmd';

const service = {
  fetchNodes(clusterId?: string) {
    return api
      .get(cfg.getClusterNodesUrl(clusterId))
      .then(json => map(json.items, makeNode));
  },

  createNodeJoinToken() {
    return api.post(cfg.getNodeJoinTokenUrl()).then(makeNodeToken);
  },

  createNodeBashCommand() {
    return this.createNodeJoinToken().then(makeNodeBashCmd);
  },

  createAppBashCommand(appName: string, appUri: string) {
    return api
      .post(cfg.getNodeJoinTokenUrl())
      .then(makeNodeToken)
      .then(token => makeAppBashCmd(token, appName, appUri));
  },
};

export default service;
