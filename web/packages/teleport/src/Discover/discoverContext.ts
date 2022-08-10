/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import Logger from 'shared/libs/logger';

import session from 'teleport/services/websession';
import serviceNodes from 'teleport/services/nodes';
import userService from 'teleport/services/user';
import appService from 'teleport/services/apps';
import desktopService from 'teleport/services/desktops';
import KubeService from 'teleport/services/kube';
import DatabaseService from 'teleport/services/databases';
import JoinTokenService from 'teleport/services/joinToken';
import { agentService } from 'teleport/services/agents';

import type { AgentIdKind } from 'teleport/services/agents';

const logger = Logger.create('teleport/discover');

export class DiscoverContext {
  // connectableAgents is a list of agent kinds that can
  // be connected by a user.
  connectableAgents: AgentIdKind[] = [];
  username = '';
  clusterId = '';

  // user + token services
  userService = userService;
  joinTokenService = new JoinTokenService();

  // resource services
  appService = appService;
  desktopService = desktopService;
  nodesService = new serviceNodes();
  kubeService = new KubeService();
  databaseService = new DatabaseService();
  agentService = agentService;

  init() {
    return userService.fetchUserContext().then(user => {
      this.username = user.username;
      this.clusterId = user.cluster.clusterId;

      const { users, tokens, nodes } = user.acl;

      // A user is able to read their own user information so we only need
      // to check for update (edit) perm to allow user to modify their user trait.
      // If a user cannot modify user traits, then they won't be able to
      // login to the agent they added.
      if (!users.edit) {
        this.connectableAgents = [];
      }

      // If a user cannot create/update provisioining tokens,
      // then they cannot add any resource.
      if (!tokens.create && !tokens.edit) {
        this.connectableAgents = [];
      }

      // Check for each agent query permissions.
      // Agent 'node' only requires the list perm, while the rest requires
      // list + read perm.
      if (nodes.list) {
        this.connectableAgents.push('node');
      }

      // TODO (anyone): remove as we implement other resources.

      // if (appServers.list && appServers.read) {
      //   this.connectableAgents.push('app');
      // }
      // if (dbServers.list && dbServers.read) {
      //   this.connectableAgents.push('db');
      // }
      // if (kubeServers.list && kubeServers.read) {
      //   this.connectableAgents.push('kube_cluster');
      // }
      // if (desktops.list && desktops.read) {
      //   this.connectableAgents.push('windows_desktop');
      // }
    });
  }

  logout() {
    session.logout();
  }
}
