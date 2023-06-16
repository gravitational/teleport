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

  getSamlIdpServiceProviderAccess() {
    return this.state.acl.samlIdpServiceProvider;
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
  // This page is only available for `dashboards`.
  hasDownloadCenterListAccess() {
    return (
      cfg.isDashboard &&
      (this.state.acl.license.read || this.state.acl.download.list)
    );
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
    return this.hasPrereqAccessToAddAgents() || this.hasAccessToQueryAgent();
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
}
