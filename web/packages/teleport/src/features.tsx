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
  AddCircle,
  Bots as BotsIcon,
  CirclePlay,
  ClipboardUser,
  Cluster,
  Integrations as IntegrationsIcon,
  Laptop,
  ListAddCheck,
  ListThin,
  Lock,
  Question,
  Server,
  ShieldCheck,
  SlidersVertical,
  Terminal,
  UserCircleGear,
  Users as UsersIcon,
} from 'design/Icon';

import cfg from 'teleport/config';

import {
  ManagementSection,
  NavigationCategory,
} from 'teleport/Navigation/categories';
import { IntegrationEnroll } from '@gravitational/teleport/src/Integrations/Enroll';

import { NavTitle } from './types';

import { AuditContainer as Audit } from './Audit';
import { SessionsContainer as Sessions } from './Sessions';
import { UnifiedResources } from './UnifiedResources';
import { AccountPage } from './Account';
import { Support } from './Support';
import { Clusters } from './Clusters';
import { Nodes } from './Nodes';
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

import type { FeatureFlags, TeleportFeature } from './types';

class AccessRequests implements TeleportFeature {
  category = NavigationCategory.Resources;

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
  };

  topMenuItem = this.navigationItem;
}

export class FeatureNodes implements TeleportFeature {
  route = {
    title: 'Servers',
    path: cfg.routes.nodes,
    exact: true,
    component: Nodes,
  };

  navigationItem = {
    title: NavTitle.Servers,
    icon: Server,
    exact: true,
    getLink(clusterId: string) {
      return cfg.getNodesRoute(clusterId);
    },
  };

  category = NavigationCategory.Resources;

  hasAccess(flags: FeatureFlags) {
    return flags.nodes;
  }
}

export class FeatureUnifiedResources implements TeleportFeature {
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
  };

  category = NavigationCategory.Resources;

  hasAccess() {
    return !cfg.isDashboard;
  }

  getRoute() {
    return this.route;
  }
}

export class FeatureSessions implements TeleportFeature {
  category = NavigationCategory.Resources;

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

  route = {
    title: 'Manage Users',
    path: cfg.routes.users,
    exact: true,
    component: () => <Users />,
  };

  hasAccess(flags: FeatureFlags): boolean {
    return flags.users;
  }

  navigationItem = {
    title: NavTitle.Users,
    icon: UsersIcon,
    exact: true,
    getLink() {
      return cfg.getUsersRoute();
    },
  };

  getRoute() {
    return this.route;
  }
}

export class FeatureBots implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Access;

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
  };

  getRoute() {
    return this.route;
  }
}

export class FeatureAddBots implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Access;
  hideFromNavigation = true;

  route = {
    title: 'New Bot',
    path: cfg.routes.botsNew,
    exact: false,
    component: () => <AddBots />,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.addBots;
  }

  getRoute() {
    return this.route;
  }
}

export class FeatureRoles implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Permissions;

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
  };
}

export class FeatureAuthConnectors implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Access;

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
    icon: ShieldCheck,
    exact: false,
    getLink() {
      return cfg.routes.sso;
    },
  };
}

export class FeatureLocks implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Identity;

  route = {
    title: 'Manage Session & Identity Locks',
    path: cfg.routes.locks,
    exact: true,
    component: Locks,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.locks;
  }

  navigationItem = {
    title: NavTitle.SessionAndIdentityLocks,
    icon: Lock,
    exact: false,
    getLink() {
      return cfg.getLocksRoute();
    },
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
  route = {
    title: 'Enroll New Resource',
    path: cfg.routes.discover,
    exact: true,
    component: Discover,
  };

  navigationItem = {
    title: NavTitle.EnrollNewResource,
    icon: AddCircle,
    exact: true,
    getLink() {
      return cfg.routes.discover;
    },
  };

  category = NavigationCategory.Management;
  section = ManagementSection.Access;

  hasAccess(flags: FeatureFlags) {
    return flags.discover;
  }

  getRoute() {
    return this.route;
  }
}

export class FeatureIntegrations implements TeleportFeature {
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
  };

  getRoute() {
    return this.route;
  }
}

export class FeatureIntegrationEnroll implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Access;

  route = {
    title: 'Enroll New Integration',
    path: cfg.routes.integrationEnroll,
    exact: false,
    component: () => <IntegrationEnroll />,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.enrollIntegrations;
  }

  navigationItem = {
    title: NavTitle.EnrollNewIntegration,
    icon: AddCircle,
    getLink() {
      return cfg.getIntegrationEnrollRoute(null);
    },
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
  };
}

export class FeatureAudit implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Activity;

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
  };
}

// - Clusters

export class FeatureClusters implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Clusters;

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
  };
}

export class FeatureTrust implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Clusters;

  route = {
    title: 'Trusted Clusters',
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
  };
}

class FeatureDeviceTrust implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Identity;
  route = {
    title: 'Manage Trusted Devices',
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
    new FeatureUnifiedResources(),
    new AccessRequests(),
    new FeatureSessions(),

    // Management

    // - Access
    new FeatureUsers(),
    new FeatureBots(),
    new FeatureAddBots(),
    new FeatureAuthConnectors(),
    new FeatureIntegrations(),
    new FeatureDiscover(),
    new FeatureIntegrationEnroll(),

    // - Permissions
    new FeatureRoles(),

    // - Identity
    new FeatureLocks(),
    new FeatureNewLock(),
    new FeatureDeviceTrust(),

    // - Activity
    new FeatureRecordings(),
    new FeatureAudit(),

    // - Clusters
    new FeatureClusters(),
    new FeatureTrust(),

    // Other
    new FeatureAccount(),
    new FeatureHelpAndSupport(),
  ];
}
