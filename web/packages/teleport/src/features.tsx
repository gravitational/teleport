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

import React from 'react';

import {
  Bots as BotsIcon,
  CirclePlay,
  ClipboardUser,
  Cluster,
  Integrations as IntegrationsIcon,
  Key,
  Laptop,
  ListAddCheck,
  ListThin,
  LockKey,
  PlugsConnected,
  Question,
  Server,
  SlidersVertical,
  Terminal,
  UserCircleGear,
  User as UserIcon,
} from 'design/Icon';

import cfg from 'teleport/config';

import {
  NavigationCategory,
  ManagementSection,
} from 'teleport/Navigation/categories';
import { NavigationCategory as SideNavigationCategory } from 'teleport/Navigation/SideNavigation/categories';
import { IntegrationEnroll } from '@gravitational/teleport/src/Integrations/Enroll';

import { NavTitle } from './types';

import { AuditContainer as Audit } from './Audit';
import { SessionsContainer as Sessions } from './Sessions';
import { UnifiedResources } from './UnifiedResources';
import { AccountPage } from './Account';
import { Support } from './Support';
import { Clusters } from './Clusters';
import { TrustedClusters } from './TrustedClusters';
import { Users } from './Users';
import { RolesContainer as Roles } from './Roles';
import { DeviceTrustLocked } from './DeviceTrust';
import { RecordingsContainer as Recordings } from './Recordings';
import { AuthConnectorsContainer as AuthConnectors } from './AuthConnectors';
import { Locks } from './LocksV2/Locks';
import { NewLockView } from './LocksV2/NewLock';
import { Discover } from './Discover';
import { LockedAccessRequests } from './AccessRequests';
import { Integrations } from './Integrations';
import { Bots } from './Bots';
import { AddBots } from './Bots/Add';
import { JoinTokens } from './JoinTokens/JoinTokens';

import type { FeatureFlags, TeleportFeature } from './types';

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
  category = NavigationCategory.Management;
  section = ManagementSection.Access;
  sideNavCategory = SideNavigationCategory.Access;
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
  category = NavigationCategory.Resources;
  sideNavCategory = SideNavigationCategory.Audit;

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
  category = NavigationCategory.Management;
  section = ManagementSection.Access;
  sideNavCategory = SideNavigationCategory.Access;

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
}

export class FeatureBots implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Access;
  sideNavCategory = SideNavigationCategory.Access;

  route = {
    title: 'Manage Bots',
    path: cfg.routes.bots,
    exact: true,
    component: Bots,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.listBots;
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

export class FeatureAddBots implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Access;
  sideNavCategory = SideNavigationCategory.AddNew;

  route = {
    title: 'Bot',
    path: cfg.routes.botsNew,
    exact: true,
    component: () => <AddBots />,
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
  category = NavigationCategory.Management;
  section = ManagementSection.Permissions;
  sideNavCategory = SideNavigationCategory.Access;

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
}

export class FeatureAuthConnectors implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Access;
  sideNavCategory = SideNavigationCategory.Access;

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
    title: NavTitle.AuthConnectors,
    icon: PlugsConnected,
    exact: false,
    getLink() {
      return cfg.routes.sso;
    },
    searchableTags: ['auth connectors', 'saml', 'okta', 'oidc', 'github'],
  };
}

export class FeatureLocks implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Identity;
  sideNavCategory = SideNavigationCategory.Identity;

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
  category = NavigationCategory.Management;
  section = ManagementSection.Access;
  sideNavCategory = SideNavigationCategory.AddNew;
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
  sideNavCategory = SideNavigationCategory.Access;
  category = NavigationCategory.Management;
  section = ManagementSection.Access;

  hasAccess(flags: FeatureFlags) {
    return flags.integrations;
  }

  route = {
    title: 'Manage Integrations',
    path: cfg.routes.integrations,
    exact: true,
    component: () => <Integrations />,
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
  category = NavigationCategory.Management;
  section = ManagementSection.Access;
  sideNavCategory = SideNavigationCategory.AddNew;

  route = {
    title: 'Integration',
    path: cfg.routes.integrationEnroll,
    exact: false,
    component: () => <IntegrationEnroll />,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.enrollIntegrations;
  }

  navigationItem = {
    title: NavTitle.EnrollNewIntegration,
    icon: IntegrationsIcon,
    getLink() {
      return cfg.getIntegrationEnrollRoute(null);
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
  category = NavigationCategory.Management;
  section = ManagementSection.Activity;
  sideNavCategory = SideNavigationCategory.Audit;

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
  category = NavigationCategory.Management;
  section = ManagementSection.Activity;
  sideNavCategory = SideNavigationCategory.Audit;

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
  category = NavigationCategory.Management;
  section = ManagementSection.Clusters;
  sideNavCategory = SideNavigationCategory.Access;

  route = {
    title: 'Clusters',
    path: cfg.routes.clusters,
    exact: false,
    component: Clusters,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.trustedClusters;
  }

  navigationItem = {
    title: NavTitle.ManageClusters,
    icon: SlidersVertical,
    exact: false,
    getLink() {
      return cfg.routes.clusters;
    },
    searchableTags: ['clusters', 'manage clusters'],
  };
}

export class FeatureTrust implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Clusters;
  sideNavCategory = SideNavigationCategory.Access;

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

class FeatureDeviceTrust implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Identity;
  sideNavCategory = SideNavigationCategory.Identity;
  route = {
    title: 'Trusted Devices',
    path: cfg.routes.deviceTrust,
    exact: true,
    component: DeviceTrustLocked,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.deviceTrust;
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
  };
}

export function getOSSFeatures(): TeleportFeature[] {
  return [
    // Resources
    // TODO(rudream): Implement shortcuts to pinned/nodes/apps/dbs/desktops/kubes.
    new FeatureUnifiedResources(),

    // AddNew
    new FeatureDiscover(),
    new FeatureIntegrationEnroll(),
    new FeatureAddBots(),

    // - Access
    new FeatureUsers(),
    new FeatureBots(),
    new FeatureJoinTokens(),
    new FeatureRoles(),
    new FeatureAuthConnectors(),
    new FeatureIntegrations(),
    new FeatureClusters(),
    new FeatureTrust(),

    // - Identity
    new AccessRequests(),
    new FeatureLocks(),
    new FeatureNewLock(),
    new FeatureDeviceTrust(),

    // - Audit
    new FeatureAudit(),
    new FeatureRecordings(),
    new FeatureSessions(),

    // Other
    new FeatureAccount(),
    new FeatureHelpAndSupport(),
  ];
}
