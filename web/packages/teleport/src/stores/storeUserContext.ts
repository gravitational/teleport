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

import { Store } from 'shared/libs/stores';

import cfg from 'teleport/config';

import { UserContext } from 'teleport/services/user';

export default class StoreUserContext extends Store<UserContext> {
  state: UserContext = null;

  isSso() {
    return this.state.authType === 'sso';
  }

  getUsername() {
    return this.state?.username;
  }

  getPasswordState() {
    return this.state.passwordState;
  }

  getClusterId() {
    return this.state.cluster.clusterId;
  }

  getEventAccess() {
    return this.state.acl.events;
  }

  getConnectorAccess() {
    return this.state.acl.authConnectors;
  }

  getRoleAccess() {
    return this.state.acl.roles;
  }

  getTrustedClusterAccess() {
    return this.state.acl.trustedClusters;
  }

  getUserAccess() {
    return this.state.acl.users;
  }

  getConnectionDiagnosticAccess() {
    return this.state.acl.connectionDiagnostic;
  }

  getAppServerAccess() {
    return this.state.acl.appServers;
  }

  getKubeServerAccess() {
    return this.state.acl.kubeServers;
  }

  getTokenAccess() {
    return this.state.acl.tokens;
  }

  getWorkflowAccess() {
    return this.state.acl.accessRequests;
  }

  getAccessStrategy() {
    return this.state.accessStrategy;
  }

  getRequestableRoles() {
    return this.state.accessCapabilities.requestableRoles;
  }

  getSuggestedReviewers() {
    return this.state.accessCapabilities.suggestedReviewers;
  }

  getBillingAccess() {
    return this.state.acl.billing;
  }

  getLockAccess() {
    return this.state.acl.lock;
  }

  getDatabaseServerAccess() {
    return this.state.acl.dbServers;
  }

  getDatabaseAccess() {
    return this.state.acl.db;
  }

  getDesktopAccess() {
    return this.state.acl.desktops;
  }

  getSessionsAccess() {
    return this.state.acl.recordedSessions;
  }

  getActiveSessionsAccess() {
    return this.state.acl.activeSessions;
  }

  getClipboardAccess() {
    return this.state.acl.clipboardSharingEnabled;
  }

  getNodeAccess() {
    return this.state.acl.nodes;
  }

  getAccessRequestId() {
    return this.state.accessRequestId;
  }

  getLicenceAccess() {
    return this.state.acl.license;
  }

  getDownloadAccess() {
    return this.state.acl.download;
  }

  getAccessRequestAccess() {
    return this.state.acl.accessRequests;
  }

  getAccessMonitoringRuleAccess() {
    return this.state.acl.accessMonitoringRule;
  }

  getAccessGraphAccess() {
    return this.state.acl.accessGraph;
  }

  // hasPrereqAccessToAddAgents checks if user meets the prerequisite
  // access to add an agent:
  //  - user should be able to create provisioning tokens
  hasPrereqAccessToAddAgents() {
    const { tokens } = this.state.acl;
    return tokens.create;
  }

  // hasDownloadCenterListAccess checks if the user
  // has access to download either teleport binaries or the license.
  // Since the page is used to download both of them, having access to one
  // is enough to show access this page.
  // This page is only available for `dashboards` and cloud customers.
  hasDownloadCenterListAccess() {
    return (
      cfg.isCloud ||
      (cfg.isDashboard &&
        (this.state.acl.license.read || this.state.acl.download.list))
    );
  }

  // hasSupportPageLinkAccess checks if the user
  // has access to a Support external link in the side menu.
  // This should only be displayed on `dashboards`.
  hasSupportPageLinkAccess() {
    return cfg.isDashboard;
  }

  // hasAccessToAgentQuery checks for at least one valid query permission.
  // Nodes require only a 'list' access while the rest of the agents
  // require 'list + read'.
  hasAccessToQueryAgent() {
    const { nodes, appServers, dbServers, kubeServers, desktops } =
      this.state.acl;

    return (
      nodes.list ||
      (appServers.read && appServers.list) ||
      (dbServers.read && dbServers.list) ||
      (kubeServers.read && kubeServers.list) ||
      (desktops.read && desktops.list)
    );
  }

  hasDiscoverAccess() {
    return (
      this.hasPrereqAccessToAddAgents() ||
      (this.hasAccessToQueryAgent() && !cfg.hideInaccessibleFeatures)
    );
  }

  getPluginsAccess() {
    return this.state.acl.plugins;
  }

  getDeviceTrustAccess() {
    return this.state.acl.deviceTrust;
  }

  getIntegrationsAccess() {
    return this.state.acl.integrations;
  }

  getAssistantAccess() {
    return this.state.acl.assist;
  }

  getAllowedSearchAsRoles() {
    return this.state.allowedSearchAsRoles;
  }

  getAccessListAccess() {
    return this.state.acl.accessList;
  }

  getAuditQueryAccess() {
    return this.state.acl.auditQuery;
  }

  getSecurityReportAccess() {
    return this.state.acl.securityReport;
  }

  getExternalAuditStorageAccess() {
    return this.state.acl.externalAuditStorage;
  }

  getBotsAccess() {
    return this.state.acl.bots;
  }
}
