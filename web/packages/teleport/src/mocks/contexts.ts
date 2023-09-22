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
import { makeAcl } from 'teleport/services/user/makeAcl';

import type { Access, Acl } from 'teleport/services/user/types';

export const noAccess: Access = {
  list: false,
  read: false,
  edit: false,
  create: false,
  remove: false,
};

export const fullAccess: Access = {
  list: true,
  read: true,
  edit: true,
  create: true,
  remove: true,
};

export const allAccessAcl: Acl = {
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
  plugins: fullAccess,
  integrations: { ...fullAccess, use: true },
  assist: fullAccess,
  samlIdpServiceProvider: fullAccess,
};

export function getAcl(cfg?: { noAccess: boolean }) {
  if (cfg?.noAccess) {
    return makeAcl({});
  }
  return makeAcl(allAccessAcl);
}

export const baseContext = {
  authType: 'local',
  userName: 'llama',
  accessCapabilities: {
    suggestedReviewers: ['george_washington@gmail.com', 'alpha'],
    requestableRoles: ['dev-a', 'dev-b', 'dev-c', 'dev-d'],
  },
  userAcl: allAccessAcl,
  cluster: {
    name: 'aws',
    lastConnected: '2020-09-26T17:30:23.512876876Z',
    status: 'online',
    nodeCount: 1,
    publicURL:
      'some-long-cluster-public-url-name.cloud.teleport.gravitational.io:1234',
    authVersion: '4.4.0-dev',
    proxyVersion: '4.4.0-dev',
  },
};

export function getUserContext() {
  return makeUserContext(baseContext);
}

export function createTeleportContext(cfg?: { customAcl?: Acl }) {
  cfg = cfg || {};
  const ctx = new TeleportContext();
  const userCtx = makeUserContext(baseContext);

  if (cfg.customAcl) {
    userCtx.acl = cfg.customAcl;
  }

  ctx.storeUser.setState(userCtx);

  return ctx;
}
