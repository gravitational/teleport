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

import { Acl } from './types';

export function makeAcl(json): Acl {
  json = json || {};
  const accessList = json.accessList || defaultAccess;
  const authConnectors = json.authConnectors || defaultAccess;
  const trustedClusters = json.trustedClusters || defaultAccess;
  const roles = json.roles || defaultAccess;
  const recordedSessions = json.recordedSessions || defaultAccess;
  const activeSessions = json.activeSessions || defaultAccess;
  const events = json.events || defaultAccess;
  const users = json.users || defaultAccess;
  const appServers = json.appServers || defaultAccess;
  const kubeServers = json.kubeServers || defaultAccess;
  const tokens = json.tokens || defaultAccess;
  const accessRequests = json.accessRequests || defaultAccess;
  const billing = json.billing || defaultAccess;
  const lock = json.lock || defaultAccess;
  const plugins = json.plugins || defaultAccess;
  const integrations = json.integrations || defaultAccessWithUse;
  const dbServers = json.dbServers || defaultAccess;
  const db = json.db || defaultAccess;
  const desktops = json.desktops || defaultAccess;
  const reviewRequests = json.reviewRequests ?? false;
  // TODO (avatus) change default to false in v19. We do not want someone
  // who _can_ access file transfers to be denied access because an older cluster
  // doesn't return the valid permission. If they don't have access, the action will
  // still fail with an error, so this is merely a UX improvment.
  const fileTransferAccess = json.fileTransferAccess ?? true; // use nullish coalescing to prevent default from overriding a strictly false value
  const connectionDiagnostic = json.connectionDiagnostic || defaultAccess;
  // Defaults to true, see RFD 0049
  // https://github.com/gravitational/teleport/blob/master/rfd/0049-desktop-clipboard.md#security
  const clipboardSharingEnabled =
    json.clipboard !== undefined ? json.clipboard : true;
  // Defaults to true, see RFD 0033
  // https://github.com/gravitational/teleport/blob/master/rfd/0033-desktop-access.md#authorization
  const desktopSessionRecordingEnabled =
    json.desktopSessionRecording !== undefined
      ? json.desktopSessionRecording
      : true;
  // Behaves like clipboardSharingEnabled, see
  // https://github.com/gravitational/teleport/pull/12684#issue-1237830087
  const directorySharingEnabled =
    json.directorySharing !== undefined ? json.directorySharing : true;

  const nodes = json.nodes || defaultAccess;
  const license = json.license || defaultAccess;
  const download = json.download || defaultAccess;

  const deviceTrust = json.deviceTrust || defaultAccess;

  const auditQuery = json.auditQuery || defaultAccess;
  const securityReport = json.securityReport || defaultAccess;

  const externalAuditStorage = json.externalAuditStorage || defaultAccess;

  const samlIdpServiceProvider = json.samlIdpServiceProvider || defaultAccess;
  const accessGraph = json.accessGraph || defaultAccess;

  const bots = json.bots || defaultAccess;
  const accessMonitoringRule = json.accessMonitoringRule || defaultAccess;

  const discoverConfigs = json.discoverConfigs || defaultAccess;

  const contacts = json.contact || defaultAccess;
  const gitServers = json.gitServers || defaultAccess;

  return {
    accessList,
    authConnectors,
    trustedClusters,
    roles,
    recordedSessions,
    activeSessions,
    events,
    users,
    appServers,
    kubeServers,
    tokens,
    accessRequests,
    reviewRequests,
    billing,
    plugins,
    integrations,
    dbServers,
    db,
    desktops,
    clipboardSharingEnabled,
    desktopSessionRecordingEnabled,
    nodes,
    directorySharingEnabled,
    connectionDiagnostic,
    license,
    download,
    deviceTrust,
    lock,
    samlIdpServiceProvider,
    auditQuery,
    securityReport,
    externalAuditStorage,
    accessGraph,
    bots,
    accessMonitoringRule,
    discoverConfigs,
    contacts,
    fileTransferAccess,
    gitServers,
  };
}

export const defaultAccess = {
  list: false,
  read: false,
  edit: false,
  create: false,
  remove: false,
};

export const defaultAccessWithUse = {
  ...defaultAccess,
  use: false,
};
