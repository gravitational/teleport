/*
Copyright 2019-2023 Gravitational, Inc.

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

import cfg from 'teleport/config';

import { StoreNav, StoreUserContext } from './stores';
import * as types from './types';
import AuditService from './services/audit';
import RecordingsService from './services/recordings';
import NodeService from './services/nodes';
import clusterService from './services/clusters';
import sessionService from './services/session';
import ResourceService from './services/resources';
import userService from './services/user';
import appService from './services/apps';
import JoinTokenService from './services/joinToken';
import KubeService from './services/kube';
import DatabaseService from './services/databases';
import desktopService from './services/desktops';
import userGroupService from './services/userGroups';
import MfaService from './services/mfa';
import { agentService } from './services/agents';
import localStorage from './services/localStorage';

class TeleportContext implements types.Context {
  // stores
  storeNav = new StoreNav();
  storeUser = new StoreUserContext();

  // services
  auditService = new AuditService();
  recordingsService = new RecordingsService();
  nodeService = new NodeService();
  clusterService = clusterService;
  sshService = sessionService;
  resourceService = new ResourceService();
  userService = userService;
  appService = appService;
  joinTokenService = new JoinTokenService();
  kubeService = new KubeService();
  databaseService = new DatabaseService();
  desktopService = desktopService;
  userGroupService = userGroupService;
  mfaService = new MfaService();

  isEnterprise = cfg.isEnterprise;
  isCloud = cfg.isCloud;
  automaticUpgradesEnabled = cfg.automaticUpgrades;
  assistEnabled = cfg.assistEnabled;
  agentService = agentService;

  // lockedFeatures are the features disabled in the user's cluster.
  // Mainly used to hide features and/or show CTAs when the user cluster doesn't support it.
  // TODO(mcbattirola): use cluster features instead of only using `isUsageBasedBilling`
  // to determine which feature is locked
  lockedFeatures: types.LockedFeatures = {
    authConnectors: cfg.isUsageBasedBilling,
    activeSessions: cfg.isUsageBasedBilling,
    accessRequests: cfg.isUsageBasedBilling,
    premiumSupport: cfg.isUsageBasedBilling,
    trustedDevices: cfg.isUsageBasedBilling,
  };

  // init fetches data required for initial rendering of components.
  // The caller of this function provides the try/catch
  // block.
  async init() {
    const user = await userService.fetchUserContext();
    this.storeUser.setState(user);

    if (
      this.storeUser.hasPrereqAccessToAddAgents() &&
      this.storeUser.hasAccessToQueryAgent() &&
      !localStorage.getOnboardDiscover()
    ) {
      const hasResource =
        await userService.checkUserHasAccessToRegisteredResource();
      localStorage.setOnboardDiscover({ hasResource });
    }
  }

  getFeatureFlags(): types.FeatureFlags {
    const userContext = this.storeUser;

    if (!this.storeUser.state) {
      return {
        activeSessions: false,
        applications: false,
        audit: false,
        authConnector: false,
        billing: false,
        databases: false,
        desktops: false,
        kubernetes: false,
        nodes: false,
        recordings: false,
        roles: false,
        trustedClusters: false,
        users: false,
        newAccessRequest: false,
        accessRequests: false,
        downloadCenter: false,
        discover: false,
        plugins: false,
        integrations: false,
        deviceTrust: false,
        enrollIntegrationsOrPlugins: false,
        enrollIntegrations: false,
        locks: false,
        newLocks: false,
        assist: false,
        samlIdpServiceProviders: false,
      };
    }

    return {
      audit: userContext.getEventAccess().list,
      recordings: userContext.getSessionsAccess().list,
      authConnector: userContext.getConnectorAccess().list,
      roles: userContext.getRoleAccess().list,
      trustedClusters: userContext.getTrustedClusterAccess().list,
      users: userContext.getUserAccess().list,
      applications: userContext.getAppServerAccess().list,
      kubernetes: userContext.getKubeServerAccess().list,
      billing: userContext.getBillingAccess().list,
      databases: userContext.getDatabaseServerAccess().list,
      desktops: userContext.getDesktopAccess().list,
      nodes: userContext.getNodeAccess().list,
      activeSessions: userContext.getActiveSessionsAccess().list,
      accessRequests: userContext.getAccessRequestAccess().list,
      newAccessRequest: userContext.getAccessRequestAccess().create,
      downloadCenter: userContext.hasDownloadCenterListAccess(),
      discover: userContext.hasDiscoverAccess(),
      plugins: userContext.getPluginsAccess().list,
      integrations: userContext.getIntegrationsAccess().list,
      enrollIntegrations: userContext.getIntegrationsAccess().create,
      enrollIntegrationsOrPlugins:
        userContext.getPluginsAccess().create ||
        userContext.getIntegrationsAccess().create,
      deviceTrust: userContext.getDeviceTrustAccess().list,
      locks: userContext.getLockAccess().list,
      newLocks:
        userContext.getLockAccess().create && userContext.getLockAccess().edit,
      assist: userContext.getAssistantAccess().list && this.assistEnabled,
      samlIdpServiceProviders:
        userContext.getSamlIdpServiceProviderAccess().list,
    };
  }
}

export default TeleportContext;
