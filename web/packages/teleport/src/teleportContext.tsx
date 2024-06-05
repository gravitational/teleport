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

import { UserPreferences } from 'gen-proto-ts/teleport/userpreferences/v1/userpreferences_pb';

import cfg from 'teleport/config';

import { StoreNav, StoreNotifications, StoreUserContext } from './stores';
import * as types from './types';
import AuditService from './services/audit';
import RecordingsService from './services/recordings';
import NodeService from './services/nodes';
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
import { storageService } from './services/storageService';
import ClustersService from './services/clusters/clusters';
import { NotificationService } from './services/notifications';
import { notificationContentFactory } from './Notifications';

class TeleportContext implements types.Context {
  // stores
  storeNav = new StoreNav();
  storeUser = new StoreUserContext();
  storeNotifications = new StoreNotifications();

  // services
  auditService = new AuditService();
  recordingsService = new RecordingsService();
  nodeService = new NodeService();
  clusterService = new ClustersService();
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
  notificationService = new NotificationService();

  notificationContentFactory = notificationContentFactory;

  isEnterprise = cfg.isEnterprise;
  isCloud = cfg.isCloud;
  automaticUpgradesEnabled = cfg.automaticUpgrades;
  automaticUpgradesTargetVersion = cfg.automaticUpgradesTargetVersion;
  assistEnabled = cfg.assistEnabled;
  agentService = agentService;
  // redirectUrl is used to redirect the user to a specific page after init.
  redirectUrl: string | null = null;

  // lockedFeatures are the features disabled in the user's cluster.
  // Mainly used to hide features and/or show CTAs when the user cluster doesn't support it.
  lockedFeatures: types.LockedFeatures = {
    authConnectors: !(cfg.oidc && cfg.saml),
    // Below should be locked for the following cases:
    //  1) feature disabled in the cluster features
    //  2) is not a legacy and igs is not enabled. legacies should have unlimited access.
    accessRequests:
      !cfg.accessRequests || (!cfg.isLegacyEnterprise() && !cfg.isIgsEnabled),
    trustedDevices:
      !cfg.trustedDevices || (!cfg.isLegacyEnterprise() && !cfg.isIgsEnabled),
  };

  // hasExternalAuditStorage indicates if an account has set up external audit storage. It is used to show or hide the External Audit Storage CTAs.
  hasExternalAuditStorage = false;

  // init fetches data required for initial rendering of components.
  // The caller of this function provides the try/catch
  // block.
  // preferences are needed in TeleportContextE, but not in TeleportContext.
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  async init(preferences: UserPreferences) {
    const user = await userService.fetchUserContext();
    this.storeUser.setState(user);

    if (
      this.storeUser.hasPrereqAccessToAddAgents() &&
      this.storeUser.hasAccessToQueryAgent() &&
      !storageService.getOnboardDiscover()
    ) {
      const hasResource =
        await userService.checkUserHasAccessToRegisteredResource();
      storageService.setOnboardDiscover({ hasResource });
    }

    if (user.acl.accessGraph.list) {
      // If access graph is enabled, check what features are enabled and store them in local storage.
      try {
        const accessGraphFeatures =
          await userService.fetchAccessGraphFeatures();

        for (let key in accessGraphFeatures) {
          window.localStorage.setItem(key, accessGraphFeatures[key]);
        }
      } catch (e) {
        // If we fail to fetch access graph features, log the error and continue.
        console.error('Failed to fetch access graph features', e);
      }
    }
  }

  getFeatureFlags(): types.FeatureFlags {
    const userContext = this.storeUser;

    if (!this.storeUser.state) {
      return disabledFeatureFlags;
    }

    // If feature hiding is enabled in the license, this returns true if the user has no list access to any feature within the management section.
    function hasManagementSectionAccess() {
      if (!cfg.hideInaccessibleFeatures) {
        return true;
      }
      return (
        userContext.getUserAccess().list ||
        userContext.getRoleAccess().list ||
        userContext.getEventAccess().list ||
        userContext.getSessionsAccess().list ||
        userContext.getTrustedClusterAccess().list ||
        userContext.getBillingAccess().list ||
        userContext.getPluginsAccess().list ||
        userContext.getIntegrationsAccess().list ||
        userContext.hasDiscoverAccess() ||
        userContext.getDeviceTrustAccess().list ||
        userContext.getLockAccess().list
      );
    }

    function hasAccessRequestsAccess() {
      // If feature hiding is enabled in the license, only allow access to access requests if the user has permission to access them, either by
      // having list access, requestable roles, or allowed search_as_roles.
      if (cfg.hideInaccessibleFeatures) {
        return !!(
          userContext.getAccessRequestAccess().list ||
          userContext.getRequestableRoles().length ||
          userContext.getAllowedSearchAsRoles().length
        );
      }

      // Return true if this isn't a Cloud dashboard cluster.
      return !cfg.isDashboard;
    }

    function hasAccessMonitoringAccess() {
      return (
        userContext.getAuditQueryAccess().list ||
        userContext.getSecurityReportAccess().list
      );
    }

    return {
      audit: userContext.getEventAccess().list,
      recordings: userContext.getSessionsAccess().list,
      authConnector: userContext.getConnectorAccess().list,
      roles: userContext.getRoleAccess().list,
      trustedClusters: userContext.getTrustedClusterAccess().list,
      users: userContext.getUserAccess().list,
      applications:
        userContext.getAppServerAccess().list &&
        userContext.getAppServerAccess().read,
      kubernetes:
        userContext.getKubeServerAccess().list &&
        userContext.getKubeServerAccess().read,
      billing: userContext.getBillingAccess().list,
      databases:
        userContext.getDatabaseServerAccess().list &&
        userContext.getDatabaseServerAccess().read,
      desktops:
        userContext.getDesktopAccess().list &&
        userContext.getDesktopAccess().read,
      nodes: userContext.getNodeAccess().list,
      activeSessions: userContext.getActiveSessionsAccess().list,
      accessRequests: hasAccessRequestsAccess(),
      newAccessRequest: userContext.getAccessRequestAccess().create,
      downloadCenter: userContext.hasDownloadCenterListAccess(),
      supportLink: userContext.hasSupportPageLinkAccess(),
      discover: userContext.hasDiscoverAccess(),
      plugins: userContext.getPluginsAccess().list,
      integrations: userContext.getIntegrationsAccess().list,
      enrollIntegrations:
        userContext.getIntegrationsAccess().create ||
        userContext.getExternalAuditStorageAccess().create,
      enrollIntegrationsOrPlugins:
        userContext.getPluginsAccess().create ||
        userContext.getIntegrationsAccess().create ||
        userContext.getExternalAuditStorageAccess().create,
      deviceTrust: userContext.getDeviceTrustAccess().list,
      locks: userContext.getLockAccess().list,
      newLocks:
        userContext.getLockAccess().create && userContext.getLockAccess().edit,
      assist: userContext.getAssistantAccess().list && this.assistEnabled,
      accessMonitoring: hasAccessMonitoringAccess(),
      managementSection: hasManagementSectionAccess(),
      accessGraph: userContext.getAccessGraphAccess().list,
      externalAuditStorage: userContext.getExternalAuditStorageAccess().list,
      listBots: userContext.getBotsAccess().list,
      addBots: userContext.getBotsAccess().create,
      editBots: userContext.getBotsAccess().edit,
      removeBots: userContext.getBotsAccess().remove,
    };
  }
}

export const disabledFeatureFlags: types.FeatureFlags = {
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
  supportLink: false,
  discover: false,
  plugins: false,
  integrations: false,
  deviceTrust: false,
  enrollIntegrationsOrPlugins: false,
  enrollIntegrations: false,
  locks: false,
  newLocks: false,
  assist: false,
  managementSection: false,
  accessMonitoring: false,
  accessGraph: false,
  externalAuditStorage: false,
  addBots: false,
  listBots: false,
  editBots: false,
  removeBots: false,
};

export default TeleportContext;
