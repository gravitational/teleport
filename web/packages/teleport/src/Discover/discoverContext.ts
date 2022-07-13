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
import cfg from 'teleport/config';
import serviceNodes from 'teleport/services/nodes';
import userService from 'teleport/services/user';
import appService from 'teleport/services/apps';
import desktopService from 'teleport/services/desktops';
import KubeService from 'teleport/services/kube';
import DatabaseService from 'teleport/services/databases';
import JoinTokenService from 'teleport/services/joinToken';

const logger = Logger.create('teleport/discover');

export class DiscoverContext {
  isEnterprise = cfg.isEnterprise;

  // user + token services
  userService = userService;
  joinTokenService = new JoinTokenService();

  // resource services
  appService = appService;
  desktopService = desktopService;
  nodesService = new serviceNodes();
  kubeService = new KubeService();
  databaseService = new DatabaseService();

  init() {
    // Fetch user context.
    // Check rbac for `role`, `token`, `<resource types*>`.
    // Create a list where all three permissions are allowed.
    // Use this list to render the agent buttons.
  }

  logout() {
    session.logout();
  }
}
