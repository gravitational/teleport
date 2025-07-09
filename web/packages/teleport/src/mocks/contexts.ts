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

import { Context as TeleportContext } from 'teleport';
import { makeAcl } from 'teleport/services/user/makeAcl';
import makeUserContext from 'teleport/services/user/makeUserContext';
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
  reviewRequests: true,
  fileTransferAccess: true,
  license: fullAccess,
  download: fullAccess,
  plugins: fullAccess,
  integrations: { ...fullAccess, use: true },
  deviceTrust: fullAccess,
  lock: fullAccess,
  samlIdpServiceProvider: fullAccess,
  accessList: fullAccess,
  auditQuery: fullAccess,
  securityReport: fullAccess,
  externalAuditStorage: fullAccess,
  accessGraph: fullAccess,
  bots: fullAccess,
  accessMonitoringRule: fullAccess,
  discoverConfigs: fullAccess,
  contacts: fullAccess,
  gitServers: fullAccess,
  accessGraphSettings: fullAccess,
  botInstances: fullAccess,
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
