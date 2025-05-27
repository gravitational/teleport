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

import {
  AddCircle,
  Bots as BotsIcon,
  CirclePlay,
  ClipboardUser,
  Cluster,
  Integrations as IntegrationsIcon,
  Key,
  Laptop,
  License,
  ListAddCheck,
  ListThin,
  ListView as ListViewIcon,
  LockKey,
  PlugsConnected,
  Question,
  Server,
  SlidersVertical,
  Terminal,
  UserCircleGear,
  User as UserIcon,
} from 'design/Icon';

import { IntegrationEnroll } from '@gravitational/teleport/src/Integrations/Enroll';
import cfg, { Cfg } from 'teleport/config';
import { IntegrationStatus } from 'teleport/Integrations/IntegrationStatus';
import {
  NavigationCategory,
  NavigationCategory as SideNavigationCategory,
} from 'teleport/Navigation/categories';

import { LockedAccessRequests } from './AccessRequests';
import { AccountPage } from './Account';
import { AuditContainer as Audit } from './Audit';
import { AuthConnectorsContainer as AuthConnectors } from './AuthConnectors';
import { BotInstances } from './BotInstances/BotInstances';
import { BotInstanceDetails } from './BotInstances/Details/BotInstanceDetails';
import { Bots } from './Bots';
import { AddBots } from './Bots/Add';
import { Clusters } from './Clusters';
import { DeviceTrustLocked } from './DeviceTrust';
import { Discover } from './Discover';
import { Integrations } from './Integrations';
import { JoinTokens } from './JoinTokens/JoinTokens';
import { Locks } from './LocksV2/Locks';
import { NewLockView } from './LocksV2/NewLock';
import { RecordingsContainer as Recordings } from './Recordings';
import { RolesContainer as Roles } from './Roles';
import { SessionsContainer as Sessions } from './Sessions';
import { Support } from './Support';
import { TrustedClusters } from './TrustedClusters';
import { NavTitle, type FeatureFlags, type TeleportFeature } from './types';
import { UnifiedResources } from './UnifiedResources';
import { Users } from './Users';
import { EmptyState as WorkloadIdentityEmptyState } from './WorkloadIdentity/EmptyState/EmptyState';

// to promote feature discoverability, most features should be visible in the navigation even if a user doesnt have access.
// However, there are some cases where hiding the feature is explicitly requested. Use this as a backdoor to hide the features that
// are usually "always visible"
export function shouldHideFromNavigation(cfg: Cfg) {
  return cfg.isDashboard || cfg.hideInaccessibleFeatures;
}

class AccessRequests implements TeleportFeature {
  category = NavigationCategory.Resources;
  sideNavCategory = SideNavigationCategory.Resources;

  route = {
    title: 'Access Requests',
    path: cfg.routes.accessRequest,
    exact: true,
    component: LockedAccessRequests,
  };

  hasAccess() {
    return true;
  }

  navigationItem = {
    title: NavTitle.AccessRequests,
    icon: ListAddCheck,
    exact: true,
    getLink() {
      return cfg.routes.accessRequest;
    },
    searchableTags: ['access requests'],
  };

  topMenuItem = this.navigationItem;
}

export class FeatureJoinTokens implements TeleportFeature {
  category = NavigationCategory.ZeroTrustAccess;

  navigationItem = {
    title: NavTitle.JoinTokens,
    icon: Key,
    exact: true,
    getLink() {
      return cfg.getJoinTokensRoute();
    },
  };

  route = {
    title: NavTitle.JoinTokens,
    path: cfg.routes.joinTokens,
    exact: true,
    component: JoinTokens,
    searchableTags: ['join tokens', 'join', 'tokens'],
  };

  hasAccess(flags: FeatureFlags): boolean {
    return flags.tokens;
  }
}

export class FeatureUnifiedResources implements TeleportFeature {
  category = NavigationCategory.Resources;
  sideNavCategory = SideNavigationCategory.Resources;
  // TODO(rudream): Remove this once shortcuts to pinned/nodes/apps/dbs/desktops/kubes are implemented.
  standalone = true;

  route = {
    title: 'Resources',
    path: cfg.routes.unifiedResources,
    exact: true,
    component: UnifiedResources,
  };

  navigationItem = {
    title: NavTitle.Resources,
    icon: Server,
    exact: true,
    getLink(clusterId: string) {
      return cfg.getUnifiedResourcesRoute(clusterId);
    },
    searchableTags: [
      'resources',
      'nodes',
      'servers',
      'applications',
      'apps',
      'desktops',
      'databases',
      'dbs',
      'kubes',
      'kubernetes',
    ],
  };

  hasAccess() {
    return !cfg.isDashboard;
  }

  getRoute() {
    return this.route;
  }
}

export class FeatureSessions implements TeleportFeature {
  category = NavigationCategory.Audit;

  route = {
    title: 'Active Sessions',
    path: cfg.routes.sessions,
    exact: true,
    component: Sessions,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.activeSessions;
  }

  navigationItem = {
    title: NavTitle.ActiveSessions,
    icon: Terminal,
    exact: true,
    getLink(clusterId: string) {
      return cfg.getSessionsRoute(clusterId);
    },
    searchableTags: ['active sessions', 'active', 'sessions'],
  };
  topMenuItem = this.navigationItem;
}

// ****************************
// Management Features
// ****************************

// - Access

export class FeatureUsers implements TeleportFeature {
  category = NavigationCategory.ZeroTrustAccess;

  route = {
    title: 'Manage Users',
    path: cfg.routes.users,
    exact: true,
    component: Users,
  };

  hasAccess(flags: FeatureFlags): boolean {
    return flags.users;
  }

  navigationItem = {
    title: NavTitle.Users,
    icon: UserIcon,
    exact: true,
    getLink() {
      return cfg.getUsersRoute();
    },
    searchableTags: ['users'],
  };

  getRoute() {
    return this.route;
  }

  showInDashboard = true;
}

export class FeatureBots implements TeleportFeature {
  category = NavigationCategory.MachineWorkloadId;

  route = {
    title: 'Manage Bots',
    path: cfg.routes.bots,
    exact: true,
    component: Bots,
  };

  hasAccess(flags: FeatureFlags) {
    // if feature hiding is enabled, only show
    // if the user has access
    if (shouldHideFromNavigation(cfg)) {
      return flags.listBots;
    }
    return true;
  }

  navigationItem = {
    title: NavTitle.Bots,
    icon: BotsIcon,
    exact: true,
    getLink() {
      return cfg.getBotsRoute();
    },
    searchableTags: ['bots'],
  };

  getRoute() {
    return this.route;
  }
}

export class FeatureBotInstances implements TeleportFeature {
  category = NavigationCategory.MachineWorkloadId;

  route = {
    title: 'View Bot instances',
    path: cfg.routes.botInstances,
    exact: true,
    component: BotInstances,
  };

  hasAccess(flags: FeatureFlags) {
    // if feature hiding is enabled, only show
    // if the user has access
    if (shouldHideFromNavigation(cfg)) {
      return flags.listBotInstances;
    }
    return true;
  }

  navigationItem = {
    title: NavTitle.BotInstances,
    icon: ListViewIcon,
    exact: true,
    getLink() {
      return cfg.getBotInstancesRoute();
    },
    searchableTags: ['bots', 'bot', 'instance', 'instances'],
  };

  getRoute() {
    return this.route;
  }
}

export class FeatureBotInstanceDetails implements TeleportFeature {
  parent = FeatureBotInstances;

  route = {
    title: 'Bot instance details',
    path: cfg.routes.botInstance,
    component: BotInstanceDetails,
  };

  hasAccess() {
    return true;
  }
}

export class FeatureAddBotsShortcut implements TeleportFeature {
  category = NavigationCategory.MachineWorkloadId;
  isHyperLink = true;

  hasAccess(flags: FeatureFlags) {
    return flags.addBots;
  }

  navigationItem = {
    title: NavTitle.NewBotShortcut,
    icon: AddCircle,
    exact: true,
    getLink() {
      return cfg.getBotsNewRoute();
    },
  };
}

export class FeatureAddBots implements TeleportFeature {
  category = NavigationCategory.AddNew;

  route = {
    title: 'Bot',
    path: cfg.routes.botsNew,
    exact: true,
    component: AddBots,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.addBots;
  }

  getRoute() {
    return this.route;
  }

  navigationItem = {
    title: NavTitle.NewBot,
    icon: BotsIcon,
    exact: true,
    getLink() {
      return cfg.getBotsNewRoute();
    },
    searchableTags: ['add bot', 'new bot', 'bots'],
  };
}

export class FeatureRoles implements TeleportFeature {
  category = NavigationCategory.ZeroTrustAccess;

  route = {
    title: 'Manage User Roles',
    path: cfg.routes.roles,
    exact: true,
    component: Roles,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.roles;
  }

  navigationItem = {
    title: NavTitle.Roles,
    icon: ClipboardUser,
    exact: true,
    getLink() {
      return cfg.routes.roles;
    },
    searchableTags: ['roles', 'user roles'],
  };

  showInDashboard = true;
  // getRoute allows child class extending this
  // parent class to refer to this parent's route.
  getRoute() {
    return this.route;
  }
}

export class FeatureAuthConnectors implements TeleportFeature {
  category = NavigationCategory.ZeroTrustAccess;

  route = {
    title: 'Manage Auth Connectors',
    path: cfg.routes.sso,
    exact: false,
    component: AuthConnectors,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.authConnector;
  }

  navigationItem = {
    title: cfg.isDashboard
      ? NavTitle.AuthConnectorsShortened
      : NavTitle.AuthConnectors,
    icon: PlugsConnected,
    exact: false,
    getLink() {
      return cfg.routes.sso;
    },
    searchableTags: ['auth connectors', 'saml', 'okta', 'oidc', 'github'],
  };

  showInDashboard = true;
}

export class FeatureLocks implements TeleportFeature {
  category = NavigationCategory.IdentityGovernance;

  route = {
    title: 'Session & Identity Locks',
    path: cfg.routes.locks,
    exact: true,
    component: Locks,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.locks;
  }

  navigationItem = {
    title: NavTitle.SessionAndIdentityLocks,
    icon: LockKey,
    exact: false,
    getLink() {
      return cfg.getLocksRoute();
    },
    searchableTags: ['locks'],
  };
}

export class FeatureNewLock implements TeleportFeature {
  route = {
    title: 'Create New Lock',
    path: cfg.routes.newLock,
    exact: true,
    component: NewLockView,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.newLocks;
  }

  // getRoute allows child class extending this
  // parent class to refer to this parent's route.
  getRoute() {
    return this.route;
  }
}

export class FeatureDiscover implements TeleportFeature {
  category = NavigationCategory.AddNew;
  standalone = true;

  route = {
    title: 'Resource',
    path: cfg.routes.discover,
    exact: true,
    component: Discover,
  };

  navigationItem = {
    title: NavTitle.EnrollNewResource,
    icon: Server,
    exact: true,
    getLink() {
      return cfg.routes.discover;
    },
    searchableTags: ['new', 'add', 'enroll', 'resources'],
  };

  hasAccess(flags: FeatureFlags) {
    return flags.discover;
  }

  getRoute() {
    return this.route;
  }
}

export class FeatureIntegrations implements TeleportFeature {
  category = NavigationCategory.ZeroTrustAccess;

  hasAccess(flags: FeatureFlags) {
    // if feature hiding is enabled, only show
    // if the user has access
    if (shouldHideFromNavigation(cfg)) {
      return flags.integrations;
    }
    return true;
  }

  route = {
    title: 'Manage Integrations',
    path: cfg.routes.integrations,
    exact: true,
    component: Integrations,
  };

  navigationItem = {
    title: NavTitle.Integrations,
    icon: IntegrationsIcon,
    exact: true,
    getLink() {
      return cfg.routes.integrations;
    },
    searchableTags: ['integrations'],
  };

  getRoute() {
    return this.route;
  }
}

export class FeatureIntegrationEnroll implements TeleportFeature {
  category = NavigationCategory.AddNew;

  route = {
    title: 'Integration',
    path: cfg.routes.integrationEnroll,
    exact: false,
    component: IntegrationEnroll,
  };

  hasAccess(flags: FeatureFlags) {
    if (shouldHideFromNavigation(cfg)) {
      return flags.enrollIntegrations;
    }
    return true;
  }

  navigationItem = {
    title: NavTitle.EnrollNewIntegration,
    icon: IntegrationsIcon,
    getLink() {
      return cfg.getIntegrationEnrollRoute();
    },
    searchableTags: ['new', 'add', 'enroll', 'integration'],
  };

  // getRoute allows child class extending this
  // parent class to refer to this parent's route.
  getRoute() {
    return this.route;
  }
}

// - Activity

export class FeatureRecordings implements TeleportFeature {
  category = NavigationCategory.Audit;

  route = {
    title: 'Session Recordings',
    path: cfg.routes.recordings,
    exact: true,
    component: Recordings,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.recordings;
  }

  navigationItem = {
    title: NavTitle.SessionRecordings,
    icon: CirclePlay,
    exact: true,
    getLink(clusterId: string) {
      return cfg.getRecordingsRoute(clusterId);
    },
    searchableTags: ['recorded sessions', 'recordings', 'sessions'],
  };
}

export class FeatureAudit implements TeleportFeature {
  category = NavigationCategory.Audit;

  route = {
    title: 'Audit Log',
    path: cfg.routes.audit,
    component: Audit,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.audit;
  }

  navigationItem = {
    title: NavTitle.AuditLog,
    icon: ListThin,
    getLink(clusterId: string) {
      return cfg.getAuditRoute(clusterId);
    },
    searchableTags: ['audit log'],
  };
}

// - Clusters

export class FeatureClusters implements TeleportFeature {
  category = NavigationCategory.ZeroTrustAccess;

  route = {
    title: 'Clusters',
    path: cfg.routes.clusters,
    exact: false,
    component: Clusters,
  };

  hasAccess(flags: FeatureFlags) {
    return cfg.isDashboard || flags.trustedClusters;
  }

  showInDashboard = true;

  navigationItem = {
    title: cfg.isDashboard
      ? NavTitle.ManageClustersShortened
      : NavTitle.ManageClusters,
    icon: SlidersVertical,
    exact: false,
    getLink() {
      return cfg.routes.clusters;
    },
    searchableTags: ['clusters', 'manage clusters'],
  };

  getRoute() {
    return this.route;
  }
}

export class FeatureTrust implements TeleportFeature {
  category = NavigationCategory.ZeroTrustAccess;

  route = {
    title: 'Trusted Root Clusters',
    path: cfg.routes.trustedClusters,
    component: TrustedClusters,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.trustedClusters;
  }

  navigationItem = {
    title: NavTitle.TrustedClusters,
    icon: Cluster,
    getLink() {
      return cfg.routes.trustedClusters;
    },
    searchableTags: ['clusters', 'trusted clusters', 'root clusters'],
  };
}

export class FeatureWorkloadIdentity implements TeleportFeature {
  category = NavigationCategory.MachineWorkloadId;
  route = {
    title: 'Workload Identity',
    path: cfg.routes.workloadIdentity,
    exact: true,
    component: WorkloadIdentityEmptyState,
  };

  // for now, workload identity page is just a placeholder so everyone has
  // access, unless feature hiding is off
  hasAccess(): boolean {
    if (shouldHideFromNavigation(cfg)) {
      return false;
    }
    return true;
  }
  navigationItem = {
    title: NavTitle.WorkloadIdentity,
    icon: License,
    getLink() {
      return cfg.routes.workloadIdentity;
    },
    searchableTags: ['workload identity', 'workload', 'identity'],
  };
}

class FeatureDeviceTrust implements TeleportFeature {
  category = NavigationCategory.IdentityGovernance;
  route = {
    title: 'Trusted Devices',
    path: cfg.routes.deviceTrust,
    exact: true,
    component: DeviceTrustLocked,
  };

  hasAccess(flags: FeatureFlags) {
    if (shouldHideFromNavigation(cfg)) {
      return flags.deviceTrust;
    }
    return true;
  }

  navigationItem = {
    title: NavTitle.TrustedDevices,
    icon: Laptop,
    exact: true,
    getLink() {
      return cfg.routes.deviceTrust;
    },
    searchableTags: ['device trust', 'trusted devices', 'devices'],
  };
}

class FeatureIntegrationStatus implements TeleportFeature {
  parent = FeatureIntegrations;

  route = {
    title: 'Integration Status',
    path: cfg.routes.integrationStatus,
    component: IntegrationStatus,
  };

  hasAccess() {
    return true;
  }
}

// ****************************
// Other Features
// ****************************

export class FeatureAccount implements TeleportFeature {
  route = {
    title: 'Account Settings',
    path: cfg.routes.account,
    component: AccountPage,
  };

  hasAccess() {
    return true;
  }

  topMenuItem = {
    title: NavTitle.AccountSettings,
    icon: UserCircleGear,
    getLink() {
      return cfg.routes.account;
    },
    searchableTags: [
      'account settings',
      'settings',
      'password',
      'mfa',
      'change password',
    ],
  };
}

export class FeatureHelpAndSupport implements TeleportFeature {
  route = {
    title: 'Help & Support',
    path: cfg.routes.support,
    exact: true,
    component: Support,
  };

  hasAccess() {
    return true;
  }

  topMenuItem = {
    title: NavTitle.HelpAndSupport,
    icon: Question,
    exact: true,
    getLink() {
      return cfg.routes.support;
    },
    searchableTags: ['help', 'support', NavTitle.HelpAndSupport],
  };
}

export function getOSSFeatures(): TeleportFeature[] {
  return [
    // Resources
    new FeatureUnifiedResources(),

    // AddNew
    new FeatureDiscover(),
    new FeatureIntegrationEnroll(),
    new FeatureAddBots(),

    // - Access
    new FeatureUsers(),
    new FeatureBots(),
    new FeatureBotInstances(),
    new FeatureBotInstanceDetails(),
    new FeatureAddBotsShortcut(),
    new FeatureJoinTokens(),
    new FeatureRoles(),
    new FeatureAuthConnectors(),
    new FeatureIntegrations(),
    new FeatureClusters(),
    new FeatureTrust(),
    new FeatureIntegrationStatus(),

    // - Identity
    new AccessRequests(),
    new FeatureLocks(),
    new FeatureNewLock(),
    new FeatureDeviceTrust(),
    new FeatureWorkloadIdentity(),

    // - Audit
    new FeatureAudit(),
    new FeatureRecordings(),
    new FeatureSessions(),

    // Other
    new FeatureAccount(),
    new FeatureHelpAndSupport(),
  ];
}
