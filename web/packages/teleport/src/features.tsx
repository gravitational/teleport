/*
Copyright 2019-2021 Gravitational, Inc.

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

import React from 'react';

import {
  ActiveSessionsIcon,
  AddIcon,
  ApplicationsIcon,
  AuditLogIcon,
  AuthConnectorsIcon,
  LockIcon,
  DatabasesIcon,
  DesktopsIcon,
  IntegrationsIcon,
  KubernetesIcon,
  ManageClustersIcon,
  RolesIcon,
  ServersIcon,
  SessionRecordingsIcon,
  SupportIcon,
  TrustedClustersIcon,
  UserSettingsIcon,
  UsersIcon,
} from 'design/SVGIcon';

import cfg from 'teleport/config';

import {
  ManagementSection,
  NavigationCategory,
} from 'teleport/Navigation/categories';

import type { TeleportFeature, FeatureFlags } from './types';

const Audit = React.lazy(
  () => import(/* webpackChunkName: "audit" */ './Audit')
);
const Nodes = React.lazy(
  () => import(/* webpackChunkName: "nodes" */ './Nodes')
);
const Sessions = React.lazy(
  () => import(/* webpackChunkName: "sessions" */ './Sessions')
);
const Account = React.lazy(
  () => import(/* webpackChunkName: "account" */ './Account')
);
const Applications = React.lazy(
  () => import(/* webpackChunkName: "apps" */ './Apps')
);
const Kubes = React.lazy(
  () => import(/* webpackChunkName: "kubes" */ './Kubes')
);
const Support = React.lazy(
  () => import(/* webpackChunkName: "support" */ './Support')
);
const Clusters = React.lazy(
  () => import(/* webpackChunkName: "clusters" */ './Clusters')
);
const Trust = React.lazy(
  () => import(/* webpackChunkName: "trusted-clusters" */ './TrustedClusters')
);
const Users = React.lazy(
  () => import(/* webpackChunkName: "users" */ './Users')
);
const Roles = React.lazy(
  () => import(/* webpackChunkName: "roles" */ './Roles')
);
const Recordings = React.lazy(
  () => import(/* webpackChunkName: "recordings" */ './Recordings')
);
const AuthConnectors = React.lazy(
  () => import(/* webpackChunkName: "auth-connectors" */ './AuthConnectors')
);
const Locks = React.lazy(
  () => import(/* webpackChunkName: "locks" */ './LocksV2/Locks')
);
const NewLock = React.lazy(
  () => import(/* webpackChunkName: "newLock" */ './LocksV2/NewLock')
);
const Databases = React.lazy(
  () => import(/* webpackChunkName: "databases" */ './Databases')
);
const Desktops = React.lazy(
  () => import(/* webpackChunkName: "desktop" */ './Desktops')
);
const Discover = React.lazy(
  () => import(/* webpackChunkName: "discover" */ './Discover')
);
const Integrations = React.lazy(
  () => import(/* webpackChunkName: "integrations" */ './Integrations')
);
const IntegrationEnroll = React.lazy(
  () =>
    import(
      /* webpackChunkName: "integration-enroll" */ '@gravitational/teleport/src/Integrations/Enroll'
    )
);

// ****************************
// Resource Features
// ****************************

export class FeatureNodes implements TeleportFeature {
  route = {
    title: 'Servers',
    path: cfg.routes.nodes,
    exact: true,
    component: Nodes,
  };

  navigationItem = {
    title: 'Servers',
    icon: <ServersIcon />,
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

export class FeatureApps implements TeleportFeature {
  category = NavigationCategory.Resources;

  route = {
    title: 'Applications',
    path: cfg.routes.apps,
    exact: true,
    component: Applications,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.applications;
  }

  navigationItem = {
    title: 'Applications',
    icon: <ApplicationsIcon />,
    exact: true,
    getLink(clusterId: string) {
      return cfg.getAppsRoute(clusterId);
    },
  };
}

export class FeatureKubes implements TeleportFeature {
  category = NavigationCategory.Resources;

  route = {
    title: 'Kubernetes',
    path: cfg.routes.kubernetes,
    exact: true,
    component: Kubes,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.kubernetes;
  }

  navigationItem = {
    title: 'Kubernetes',
    icon: <KubernetesIcon />,
    exact: true,
    getLink(clusterId: string) {
      return cfg.getKubernetesRoute(clusterId);
    },
  };
}

export class FeatureDatabases implements TeleportFeature {
  category = NavigationCategory.Resources;

  route = {
    title: 'Databases',
    path: cfg.routes.databases,
    exact: true,
    component: Databases,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.databases;
  }

  navigationItem = {
    title: 'Databases',
    icon: <DatabasesIcon />,
    exact: true,
    getLink(clusterId: string) {
      return cfg.getDatabasesRoute(clusterId);
    },
  };
}

export class FeatureDesktops implements TeleportFeature {
  category = NavigationCategory.Resources;

  route = {
    title: 'Desktops',
    path: cfg.routes.desktops,
    exact: true,
    component: Desktops,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.desktops;
  }

  navigationItem = {
    title: 'Desktops',
    icon: <DesktopsIcon />,
    exact: true,
    getLink(clusterId: string) {
      return cfg.getDesktopsRoute(clusterId);
    },
  };
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
    title: 'Active Sessions',
    icon: <ActiveSessionsIcon />,
    exact: true,
    getLink(clusterId: string) {
      return cfg.getSessionsRoute(clusterId);
    },
  };
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
    component: Users,
  };

  hasAccess(flags: FeatureFlags): boolean {
    return flags.users;
  }

  navigationItem = {
    title: 'Users',
    icon: <UsersIcon />,
    exact: true,
    getLink() {
      return cfg.getUsersRoute();
    },
  };
}

export class FeatureRoles implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Access;

  route = {
    title: 'Manage Roles',
    path: cfg.routes.roles,
    exact: true,
    component: Roles,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.roles;
  }

  navigationItem = {
    title: 'Roles',
    icon: <RolesIcon />,
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
    title: 'Auth Connectors',
    icon: <AuthConnectorsIcon />,
    exact: false,
    getLink() {
      return cfg.routes.sso;
    },
  };
}

export class FeatureLocks implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Access;

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
    title: 'Session & Identity Locks',
    icon: <LockIcon />,
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
    component: NewLock,
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
    title: 'Enroll New Resource',
    icon: <AddIcon />,
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
    title: 'Integrations',
    icon: <IntegrationsIcon />,
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
    title: 'Enroll New Integration',
    icon: <AddIcon />,
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
    title: 'Session Recordings',
    icon: <SessionRecordingsIcon />,
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
    title: 'Audit Log',
    icon: <AuditLogIcon />,
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
    title: 'Manage Clusters',
    icon: <ManageClustersIcon />,
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
    component: Trust,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.trustedClusters;
  }

  navigationItem = {
    title: 'Trusted Clusters',
    icon: <TrustedClustersIcon />,
    getLink() {
      return cfg.routes.trustedClusters;
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
    component: Account,
  };

  hasAccess() {
    return true;
  }

  topMenuItem = {
    title: 'Account Settings',
    icon: <UserSettingsIcon size={16} />,
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
    title: 'Help & Support',
    icon: <SupportIcon size={16} />,
    exact: true,
    getLink() {
      return cfg.routes.support;
    },
  };
}

export function getOSSFeatures(): TeleportFeature[] {
  return [
    // Resources
    new FeatureNodes(),
    new FeatureApps(),
    new FeatureKubes(),
    new FeatureDatabases(),
    new FeatureDesktops(),
    new FeatureSessions(),

    // Management

    // - Access
    new FeatureUsers(),
    new FeatureRoles(),
    new FeatureAuthConnectors(),
    new FeatureLocks(),
    new FeatureNewLock(),
    new FeatureIntegrations(),
    new FeatureDiscover(),
    new FeatureIntegrationEnroll(),

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
