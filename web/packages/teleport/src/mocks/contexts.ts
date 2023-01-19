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

import makeUserContext from 'teleport/services/user/makeUserContext';
import { Context as TeleportContext } from 'teleport';

import type { Access, Acl } from 'teleport/services/user/types';

export const fullAccess: Access = {
  list: true,
  read: true,
  edit: true,
  create: true,
  remove: true,
};

export const fullAcl: Acl = {
  windowsLogins: ['Administrator'],
  tokens: fullAccess,
  appServers: fullAccess,
  kubeServers: fullAccess,
  recordedSessions: fullAccess,
  activeSessions: fullAccess,
  authConnectors: fullAccess,
  roles: fullAccess,
  users: fullAccess,
  trustedClusters: fullAccess,
  events: fullAccess,
  accessRequests: fullAccess,
  billing: fullAccess,
  dbServers: fullAccess,
  db: fullAccess,
  desktops: fullAccess,
  nodes: fullAccess,
  connectionDiagnostic: fullAccess,
  clipboardSharingEnabled: true,
  desktopSessionRecordingEnabled: true,
  directorySharingEnabled: true,
  license: fullAccess,
  download: fullAccess,
};

export const userContext = makeUserContext({
  authType: 'sso',
  userName: 'llama',
  accessCapabilities: {
    suggestedReviewers: ['george_washington@gmail.com', 'alpha'],
    requestableRoles: ['dev-a', 'dev-b', 'dev-c', 'dev-d'],
  },
  userAcl: fullAcl,
  cluster: {
    name: 'aws',
    lastConnected: '2020-09-26T17:30:23.512876876Z',
    status: 'online',
    nodeCount: 1,
    publicURL: 'localhost',
    authVersion: '4.4.0-dev',
    proxyVersion: '4.4.0-dev',
  },
});

export const baseContext = {
  authType: 'local',
  userName: 'llama',
  accessCapabilities: {
    suggestedReviewers: ['george_washington@gmail.com', 'alpha'],
    requestableRoles: ['dev-a', 'dev-b', 'dev-c', 'dev-d'],
  },
  userAcl: fullAcl,
  cluster: {
    name: 'aws',
    lastConnected: '2020-09-26T17:30:23.512876876Z',
    status: 'online',
    nodeCount: 1,
    publicURL: 'localhost',
    authVersion: '4.4.0-dev',
    proxyVersion: '4.4.0-dev',
  },
};

export function createTeleportContext() {
  const ctx = new TeleportContext();
  const userCtx = makeUserContext(baseContext);

  ctx.storeUser.setState(userCtx);

  return ctx;
}
