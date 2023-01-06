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

import * as Icons from 'design/Icon';

import Ctx from 'teleport/teleportContext';
import cfg from 'teleport/config';

import { Feature } from './types';

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
const Databases = React.lazy(
  () => import(/* webpackChunkName: "databases" */ './Databases')
);
const Desktops = React.lazy(
  () => import(/* webpackChunkName: "desktop" */ './Desktops')
);

export class FeatureClusters extends Feature {
  topNavTitle = 'Clusters';

  route = {
    title: 'Clusters',
    path: cfg.routes.clusters,
    exact: false,
    component: Clusters,
  };

  isAvailable(ctx: Ctx): boolean {
    return ctx.getFeatureFlags().trustedClusters;
  }

  register(ctx: Ctx) {
    ctx.storeNav.addSideItem({
      title: 'Manage Clusters',
      group: 'clusters',
      Icon: Icons.EqualizerVertical,
      exact: false,
      getLink() {
        return cfg.routes.clusters;
      },
    });

    ctx.features.push(this);
  }
}

export class FeatureAuthConnectors extends Feature {
  topNavTitle = 'Team';

  route = {
    title: 'Auth Connectors',
    path: cfg.routes.sso,
    exact: false,
    component: AuthConnectors,
  };

  isAvailable(ctx: Ctx): boolean {
    return ctx.getFeatureFlags().authConnector;
  }

  register(ctx: Ctx) {
    ctx.storeNav.addSideItem({
      group: 'team',
      title: 'Auth Connectors',
      Icon: Icons.Lock,
      exact: false,
      getLink() {
        return cfg.routes.sso;
      },
    });

    ctx.features.push(this);
  }
}

export class FeatureHelpAndSupport extends Feature {
  topNavTitle = 'Help & Support';

  route = {
    title: 'Help & Support',
    path: cfg.routes.support,
    exact: true,
    component: Support,
  };

  isAvailable(): boolean {
    return true;
  }

  register(ctx: Ctx) {
    ctx.storeNav.addTopMenuItem({
      title: 'Help & Support',
      Icon: Icons.Question,
      exact: true,
      getLink() {
        return cfg.routes.support;
      },
    });

    ctx.features.push(this);
  }
}

export class FeatureAudit extends Feature {
  topNavTitle = 'Account Settings';

  route = {
    title: 'Audit Log',
    path: cfg.routes.audit,
    component: Audit,
  };

  isAvailable(ctx: Ctx): boolean {
    return ctx.getFeatureFlags().audit;
  }

  register(ctx: Ctx) {
    ctx.storeNav.addSideItem({
      group: 'activity',
      title: 'Audit Log',
      Icon: Icons.ListThin,
      getLink(clusterId: string) {
        return cfg.getAuditRoute(clusterId);
      },
    });

    ctx.features.push(this);
  }
}

export class FeatureAccount extends Feature {
  topNavTitle = 'Account Settings';

  route = {
    title: 'Account Settings',
    path: cfg.routes.account,
    component: Account,
  };

  isAvailable(): boolean {
    return true;
  }

  register(ctx: Ctx) {
    ctx.storeNav.addTopMenuItem({
      title: 'Account Settings',
      Icon: Icons.Cog,
      getLink() {
        return cfg.routes.account;
      },
    });

    ctx.features.push(this);
  }
}

export class FeatureNodes extends Feature {
  topNavTitle = '';

  route = {
    title: 'Servers',
    path: cfg.routes.nodes,
    exact: true,
    component: Nodes,
  };

  isAvailable(ctx: Ctx): boolean {
    return ctx.getFeatureFlags().nodes;
  }

  register(ctx: Ctx) {
    ctx.storeNav.addSideItem({
      title: 'Servers',
      Icon: Icons.Server,
      exact: true,
      getLink(clusterId: string) {
        return cfg.getNodesRoute(clusterId);
      },
    });

    ctx.features.push(this);
  }
}

export class FeatureRecordings extends Feature {
  topNavTitle = '';

  route = {
    title: 'Session Recordings',
    path: cfg.routes.recordings,
    exact: true,
    component: Recordings,
  };

  isAvailable(ctx: Ctx): boolean {
    return ctx.getFeatureFlags().recordings;
  }

  register(ctx: Ctx) {
    ctx.storeNav.addSideItem({
      group: 'activity',
      title: 'Session Recordings',
      Icon: Icons.CirclePlay,
      exact: true,
      getLink(clusterId: string) {
        return cfg.getRecordingsRoute(clusterId);
      },
    });

    ctx.features.push(this);
  }
}

export class FeatureSessions extends Feature {
  topNavTitle = 'Sessions';

  route = {
    title: 'Sessions',
    path: cfg.routes.sessions,
    exact: true,
    component: Sessions,
  };

  isAvailable(ctx: Ctx): boolean {
    return ctx.getFeatureFlags().activeSessions;
  }

  register(ctx: Ctx) {
    ctx.storeNav.addSideItem({
      group: 'activity',
      title: 'Active Sessions',
      Icon: Icons.Terminal,
      exact: true,
      getLink(clusterId: string) {
        return cfg.getSessionsRoute(clusterId);
      },
    });

    ctx.features.push(this);
  }
}

export class FeatureRoles extends Feature {
  topNavTitle = 'Team';

  route = {
    title: 'Roles',
    path: cfg.routes.roles,
    exact: true,
    component: Roles,
  };

  isAvailable(ctx: Ctx): boolean {
    return ctx.getFeatureFlags().roles;
  }

  register(ctx: Ctx) {
    ctx.storeNav.addSideItem({
      title: 'Roles',
      group: 'team',
      Icon: Icons.ClipboardUser,
      exact: true,
      getLink() {
        return cfg.routes.roles;
      },
    });

    ctx.features.push(this);
  }
}

export class FeatureUsers extends Feature {
  topNavTitle = 'Team';

  route = {
    title: 'Users',
    path: cfg.routes.users,
    exact: true,
    component: Users,
  };

  isAvailable(ctx: Ctx): boolean {
    return ctx.getFeatureFlags().users;
  }

  register(ctx: Ctx) {
    ctx.storeNav.addSideItem({
      title: 'Users',
      group: 'team',
      Icon: Icons.Users,
      exact: true,
      getLink() {
        return cfg.routes.users;
      },
    });

    ctx.features.push(this);
  }
}

export class FeatureApps extends Feature {
  topNavTitle = 'Applications';

  route = {
    title: 'Applications',
    path: cfg.routes.apps,
    exact: true,
    component: Applications,
  };

  isAvailable(ctx: Ctx): boolean {
    return ctx.getFeatureFlags().applications;
  }

  register(ctx: Ctx) {
    ctx.storeNav.addSideItem({
      title: 'Applications',
      Icon: Icons.NewTab,
      exact: true,
      getLink(clusterId) {
        return cfg.getAppsRoute(clusterId);
      },
    });

    ctx.features.push(this);
  }
}

export class FeatureKubes extends Feature {
  topNavTitle = '';

  route = {
    title: 'Kubernetes',
    path: cfg.routes.kubernetes,
    exact: true,
    component: Kubes,
  };

  isAvailable(ctx: Ctx): boolean {
    return ctx.getFeatureFlags().kubernetes;
  }

  register(ctx: Ctx) {
    ctx.storeNav.addSideItem({
      title: 'Kubernetes',
      Icon: Icons.Kubernetes,
      exact: true,
      getLink(clusterId: string) {
        return cfg.getKubernetesRoute(clusterId);
      },
    });

    ctx.features.push(this);
  }
}

export class FeatureTrust extends Feature {
  topNavTitle = 'Clusters';

  route = {
    title: 'Trust',
    path: cfg.routes.trustedClusters,
    component: Trust,
  };

  isAvailable(ctx: Ctx): boolean {
    return ctx.getFeatureFlags().trustedClusters;
  }

  register(ctx: Ctx) {
    ctx.storeNav.addSideItem({
      group: 'clusters',
      title: 'Trust',
      Icon: Icons.Cluster,
      getLink() {
        return cfg.routes.trustedClusters;
      },
    });

    ctx.features.push(this);
  }
}

export class FeatureDatabases extends Feature {
  topNavTitle = '';

  route = {
    title: 'Databases',
    path: cfg.routes.databases,
    exact: true,
    component: Databases,
  };

  isAvailable(ctx: Ctx): boolean {
    return ctx.getFeatureFlags().databases;
  }

  register(ctx: Ctx) {
    ctx.storeNav.addSideItem({
      title: 'Databases',
      Icon: Icons.Database,
      exact: true,
      getLink(clusterId: string) {
        return cfg.getDatabasesRoute(clusterId);
      },
    });

    ctx.features.push(this);
  }
}

export class FeatureDesktops extends Feature {
  topNavTitle = '';

  route = {
    title: 'Desktops',
    path: cfg.routes.desktops,
    exact: true,
    component: Desktops,
  };

  isAvailable(ctx: Ctx): boolean {
    return ctx.getFeatureFlags().desktops;
  }

  register(ctx: Ctx) {
    ctx.storeNav.addSideItem({
      title: 'Desktops',
      Icon: Icons.Desktop,
      exact: true,
      getLink(clusterId: string) {
        return cfg.getDesktopsRoute(clusterId);
      },
    });

    ctx.features.push(this);
  }
}

export function getOSSFeatures(): Feature[] {
  return [
    new FeatureNodes(),
    new FeatureApps(),
    new FeatureKubes(),
    new FeatureDatabases(),
    new FeatureDesktops(),
    new FeatureSessions(),
    new FeatureRecordings(),
    new FeatureAudit(),
    new FeatureUsers(),
    new FeatureRoles(),
    new FeatureAuthConnectors(),
    new FeatureAccount(),
    new FeatureHelpAndSupport(),
    new FeatureClusters(),
    new FeatureTrust(),
  ];
}
