import React from 'react';

import * as Icons from 'design/Icon';
import * as OSS from 'teleport/features';

import {
  NavigationCategory,
  ManagementSection,
} from 'teleport/Navigation/categories';

import {
  AccessRequestsIcon,
  DownloadsIcon,
  SupportIcon,
  DevicesIcon,
  IntegrationsIcon,
} from 'design/SVGIcon';

import cfg from 'e-teleport/config';
import { ReviewRequests, NewRequest } from 'e-teleport/Workflow';

import { Downloads } from 'e-teleport/Downloads';

import type { TeleportFeature, FeatureFlags } from 'teleport/types';

const AuthConnectors = React.lazy(
  () =>
    import(
      /* webpackChunkName: "e-auth-connectors" */ 'e-teleport/AuthConnectors'
    )
);
const AccountE = React.lazy(
  () => import(/* webpackChunkName: "e-account" */ 'e-teleport/Account')
);
const Plugins = React.lazy(
  () => import(/* webpackChunkName: "e-plugins" */ 'e-teleport/Plugins')
);
const PluginEnroll = React.lazy(
  () =>
    import(
      /* webpackChunkName: "e-plugins" */ 'e-teleport/Plugins/PluginEnroll'
    )
);
const SupportE = React.lazy(
  () => import(/* webpackChunkName: "e-support" */ 'e-teleport/Support')
);
const NewLock = React.lazy(
  () => import(/* webpackChunkName: "new-lock" */ 'e-teleport/NewLock')
);

const DeviceTrust = React.lazy(
  () => import(/* webpackChunkName: "e-devices" */ 'e-teleport/DeviceTrust')
);

// ****************************
// Resource Features
// ****************************

class FeatureAccessRequests implements TeleportFeature {
  category = NavigationCategory.Resources;

  hasAccess() {
    return !cfg.oss.isDashboard;
  }

  navigationItem = {
    title: 'Access Requests',
    icon: <AccessRequestsIcon />,
  };
}

class FeatureNewAccessRequest implements TeleportFeature {
  category = NavigationCategory.Resources;
  parent = FeatureAccessRequests;

  route = {
    title: 'New Request',
    path: cfg.routes.requestNew,
    component: NewRequest,
  };

  hasAccess() {
    return !cfg.oss.isDashboard;
  }

  navigationItem = {
    title: 'New Request',
    icon: <Icons.Add />,
    getLink(clusterId: string) {
      return cfg.getNewAccessRequestRoute(clusterId);
    },
  };
}

class FeatureReviewAccessRequests implements TeleportFeature {
  category = NavigationCategory.Resources;

  parent = FeatureAccessRequests;

  route = {
    title: 'Review Requests',
    path: cfg.routes.requests,
    component: ReviewRequests,
  };

  hasAccess() {
    return !cfg.oss.isDashboard;
  }

  navigationItem = {
    title: 'Review Requests',
    icon: <Icons.ListAddCheck />,
    getLink() {
      return cfg.getAccessRequestRoute();
    },
  };
}

export class FeatureNewLock implements TeleportFeature {
  route = {
    title: 'Create New Lock',
    path: cfg.oss.routes.newLock,
    exact: true,
    component: NewLock,
  };

  hasAccess() {
    return true;
  }
}

// ****************************
// Legacy links in the navigation for Houston
// ****************************

class FeatureDownloadCenter implements TeleportFeature {
  category = NavigationCategory.Resources;

  route = {
    title: 'Downloads',
    path: cfg.routes.downloadCenter,
    exact: true,
    component: Downloads,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.downloadCenter;
  }

  navigationItem = {
    title: 'Downloads',
    icon: <DownloadsIcon size={22} />,
    getLink() {
      return cfg.routes.downloadCenter;
    },
  };
}

class FeatureSupport implements TeleportFeature {
  category = NavigationCategory.Resources;

  hasAccess(flags: FeatureFlags) {
    return flags.downloadCenter; // use the same flag as the download center to hide for non-Houston deployments
  }

  navigationItem = {
    title: 'Support',
    icon: <SupportIcon size={22} />,
    getLink() {
      return 'https://support.goteleport.com/';
    },
    isExternalLink: true,
  };
}

// ****************************
// Management Features
// ****************************

class FeatureAuthConnectors extends OSS.FeatureAuthConnectors {
  route = {
    title: 'Manage Auth Connectors',
    path: cfg.oss.routes.sso,
    exact: false,
    component: AuthConnectors,
  };
}

class FeatureDeviceTrust implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Access;
  route = {
    title: 'Manage Trusted Devices',
    path: cfg.routes.deviceTrust,
    exact: true,
    component: DeviceTrust,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.deviceTrust;
  }

  navigationItem = {
    title: 'Trusted Devices',
    icon: <DevicesIcon />,
    exact: true,
    getLink() {
      return cfg.routes.deviceTrust;
    },
  };
}
class FeatureIntegrations implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Access;

  hasAccess(flags: FeatureFlags) {
    return flags.plugins;
  }

  route = {
    title: 'Manage Integrations',
    path: cfg.routes.integrations,
    exact: true,
    component: () => <Plugins />,
  };

  navigationItem = {
    title: 'Integrations',
    icon: <IntegrationsIcon />,
    exact: true,
    getLink() {
      return cfg.routes.integrations;
    },
  };
}

class FeatureNewIntegration implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Access;

  route = {
    title: 'Enroll New Integration',
    path: cfg.routes.integrationEnroll,
    exact: false,
    component: () => <PluginEnroll />,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.plugins;
  }

  navigationItem = {
    title: 'Enroll New Integration',
    icon: <Icons.Add />,
    exact: false,
    getLink() {
      return cfg.getIntegrationEnrollRoute(null);
    },
  };
}

// ****************************
// Other Features
// ****************************

class FeatureAccount extends OSS.FeatureAccount {
  route = {
    title: 'Account Settings',
    path: cfg.oss.routes.account,
    component: AccountE,
  };
}

class FeatureHelpAndSupport extends OSS.FeatureHelpAndSupport {
  route = {
    title: 'Help & Support',
    path: cfg.oss.routes.support,
    exact: true,
    component: SupportE,
  };
}

export function getEnterpriseFeatures(): TeleportFeature[] {
  return [
    // Resources
    new OSS.FeatureNodes(),
    new OSS.FeatureApps(),
    new OSS.FeatureKubes(),
    new OSS.FeatureDatabases(),
    new OSS.FeatureDesktops(),
    new FeatureAccessRequests(),
    new FeatureNewAccessRequest(),
    new FeatureReviewAccessRequests(),
    new OSS.FeatureSessions(),

    // Legacy links in the navigation for Houston
    new FeatureDownloadCenter(),
    new FeatureSupport(),

    // Management

    // - Access
    new OSS.FeatureUsers(),
    new OSS.FeatureRoles(),
    new FeatureDeviceTrust(),
    new FeatureAuthConnectors(),
    new OSS.FeatureLocks(),
    new FeatureNewLock(),
    new FeatureIntegrations(),
    new OSS.FeatureDiscover(),
    new FeatureNewIntegration(),

    // - Activity
    new OSS.FeatureRecordings(),
    new OSS.FeatureAudit(),

    // - Clusters
    new OSS.FeatureClusters(),
    new OSS.FeatureTrust(),

    // Other
    new FeatureAccount(),
    new FeatureHelpAndSupport(),
  ];
}
