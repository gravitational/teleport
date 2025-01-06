import { lazy } from 'react';

import {
  Add,
  Chart,
  Code,
  Crown,
  Download,
  Graph,
  Headset,
  Laptop,
  Layout,
  LineSegments,
  ListAddCheck,
  Plugs,
  Table,
  UserList,
} from 'design/Icon';
import { PassthroughPage } from 'shared/components/AuthorizeDeviceWeb/AuthorizeDeviceWeb';

import { AccessListManagement } from 'e-teleport/AccessListManagement';
import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { CreateAccessList } from 'e-teleport/AccessListManagement/CreateAccessList';
import { AccessMonitoring } from 'e-teleport/AccessMonitoring';
import { Account as AccountE } from 'e-teleport/Account';
import { AuthConnectors } from 'e-teleport/AuthConnectors';
import { Clusters as ClustersE } from 'e-teleport/Clusters';
import cfg from 'e-teleport/config';
import { DeviceTrust } from 'e-teleport/DeviceTrust';
import { Discover as DiscoverE } from 'e-teleport/Discover';
import { Downloads } from 'e-teleport/Downloads';
import Integrations from 'e-teleport/Integrations';
import { IntegrationEnroll } from 'e-teleport/Integrations/IntegrationEnroll';
import { IntegrationStatus } from 'e-teleport/Integrations/IntegrationStatus';
import { NewLock } from 'e-teleport/NewLockV2';
import { SSOConfirm } from 'e-teleport/SSOConfirm/SSOConfirm';
import SupportE from 'e-teleport/Support';
import { UnifiedResourcesE } from 'e-teleport/UnifiedResources';
import UsageSummary from 'e-teleport/UsageSummary';
import { Users } from 'e-teleport/Users';
import NewRequest from 'e-teleport/Workflow/NewRequest/NewRequest';
import ReviewRequests from 'e-teleport/Workflow/ReviewRequests/ReviewRequests';
import * as OSS from 'teleport/features';
import {
  ManagementSection,
  NavigationCategory,
} from 'teleport/Navigation/categories';
import { NavigationCategory as SideNavigationCategory } from 'teleport/Navigation/SideNavigation/categories';
import { storageService } from 'teleport/services/storageService';
import {
  NavTitle,
  type FeatureFlags,
  type TeleportFeature,
  type TeleportFeatureNavigationItem,
  type TeleportFeatureRoute,
} from 'teleport/types';

// ****************************
// Resource Features
// ****************************

const AccessGraph = lazy(() => import('e-teleport/AccessGraph'));

class FeatureUnifiedResources extends OSS.FeatureUnifiedResources {
  route = {
    ...super.getRoute(),
    // Enterprise Unified Resources can display requestable resources
    // and allows the creation of access requests
    component: UnifiedResourcesE,
  };
}

class FeatureAccessRequests implements TeleportFeature {
  category = NavigationCategory.Resources;
  sideNavCategory = SideNavigationCategory.Identity;

  route: TeleportFeatureRoute; // intentionally undefined

  navigationItem: TeleportFeatureNavigationItem = {
    isSelected: (clusterId: string, pathname: string) => {
      return (
        pathname === cfg.getAccessRequestRoute() ||
        pathname === cfg.getNewAccessRequestRoute(clusterId)
      );
    },
    title: NavTitle.AccessRequests,
    icon: ListAddCheck,
    getLink() {
      return cfg.getAccessRequestRoute();
    },
    searchableTags: ['access requests'],
  };

  hasAccess(flags: FeatureFlags) {
    return flags.accessRequests;
  }

  topMenuItem = this.navigationItem;
}

class FeatureNewAccessRequest implements TeleportFeature {
  category = NavigationCategory.Resources;
  sideNavCategory = SideNavigationCategory.Identity;

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
    title: NavTitle.NewRequest,
    icon: Add,
    getLink(clusterId: string) {
      return cfg.getNewAccessRequestRoute(clusterId);
    },
  };
}

class FeatureReviewAccessRequests implements TeleportFeature {
  category = NavigationCategory.Resources;
  sideNavCategory = SideNavigationCategory.Identity;

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
    title: NavTitle.ReviewRequests,
    icon: ListAddCheck,
    getLink() {
      return cfg.getAccessRequestRoute();
    },
  };
}

class FeatureNewLock extends OSS.FeatureNewLock {
  route = {
    ...super.getRoute(),
    // Enterprise version allows resources access requests
    // and device trusts to be locked.
    component: NewLock,
  };
}

export class FeatureDiscoverE extends OSS.FeatureDiscover {
  route = {
    ...super.getRoute(),
    component: DiscoverE,
  };
}

// ****************************
//  Billing Features
// ****************************

export class FeatureUsageSummary implements TeleportFeature {
  section = ManagementSection.Billing;

  route = {
    title: 'Usage Tracking',
    path: cfg.routes.usageSummarySummary,
    component: UsageSummary,
  };

  hasAccess(flags: FeatureFlags) {
    return (
      flags.billing && cfg.oss.isUsageBasedBilling && !cfg.oss.isStripeManaged
    );
  }

  topMenuItem = {
    title: NavTitle.BillingSummary,
    icon: Chart,
    exact: true,
    getLink(clusterId: string) {
      return cfg.getUsageSummarySummaryRoute(clusterId);
    },
  };
}

// ****************************
// Legacy links in the navigation for Houston
// ****************************

class FeatureDownloadCenter implements TeleportFeature {
  route = {
    title: 'Downloads',
    path: cfg.oss.routes.downloadCenter,
    exact: true,
    component: Downloads,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.downloadCenter;
  }

  navigationItem = {
    title: NavTitle.Downloads,
    icon: Download,
    getLink() {
      return cfg.oss.routes.downloadCenter;
    },
  };
  topMenuItem = this.navigationItem;
}

class FeatureSupport implements TeleportFeature {
  category = NavigationCategory.Resources;

  hasAccess(flags: FeatureFlags) {
    return flags.supportLink;
  }

  navigationItem = {
    title: NavTitle.Support,
    icon: Headset,
    getLink() {
      return 'https://support.goteleport.com/';
    },
    isExternalLink: true,
  };
}

// ****************************
// Management Features
// ****************************

class FeatureAccessMonitoring implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Identity;
  sideNavCategory = SideNavigationCategory.Identity;

  route = {
    title: 'Access Monitoring',
    path: cfg.routes.accessMonitoring.base,
    exact: false,
    component: AccessMonitoring,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.accessMonitoring;
  }

  navigationItem = {
    title: NavTitle.AccessMonitoring,
    icon: Graph,
    getLink() {
      return cfg.routes.accessMonitoring.base;
    },
    searchableTags: ['access monitoring'],
  };
}

class FeatureAuthConnectors extends OSS.FeatureAuthConnectors {
  route = {
    title: 'Manage Auth Connectors',
    path: cfg.oss.routes.sso,
    exact: false,
    component: AuthConnectors,
  };
}

class FeatureAccessListManagement implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Identity;
  sideNavCategory = SideNavigationCategory.Identity;

  route = {
    title: 'Manage Access Lists',
    path: cfg.routes.accessLists,
    exact: false,
    component: AccessListManagement,
  };

  // Hide if this is a self-hosted dashboard tenant
  hasAccess() {
    return !cfg.oss.isDashboard;
  }

  navigationItem = {
    title: NavTitle.AccessLists,
    icon: UserList,
    getLink() {
      return cfg.getAccessListManagementRoute(null);
    },
    searchableTags: ['access lists', 'lists'],
  };
}

class FeatureNewAccessList implements TeleportFeature {
  sideNavCategory = SideNavigationCategory.AddNew;

  route = {
    title: NavTitle.NewAccessList,
    path: cfg.routes.accessListNew,
    exact: true,
    component: () => (
      <AccessListManagementContextProvider>
        <CreateAccessList />
      </AccessListManagementContextProvider>
    ),
  };

  // Hide if this is a self-hosted dashboard tenant
  hasAccess() {
    return !cfg.oss.isDashboard;
  }

  navigationItem = {
    title: NavTitle.NewAccessList,
    icon: UserList,
    getLink() {
      return cfg.routes.accessListNew;
    },
    searchableTags: ['new access lists', 'add access list', 'lists'],
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
    component: DeviceTrust,
  };

  hasAccess(flags: FeatureFlags) {
    if (cfg.oss.hideInaccessibleFeatures) {
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

class FeatureIntegrations extends OSS.FeatureIntegrations {
  category = NavigationCategory.Management;

  route = {
    ...super.getRoute(),
    // Enterprise version includes the enterprise only
    //  "plugin" resource along with the base
    // "integration" resource.
    component: () => <Integrations />,
  };

  hasAccess(flags: FeatureFlags) {
    // if feature hiding is enabled, only show
    // if the user has access
    if (cfg.oss.hideInaccessibleFeatures) {
      return flags.plugins || flags.integrations || flags.externalAuditStorage;
    }
    return true;
  }
}

class FeatureIntegrationEnroll extends OSS.FeatureIntegrationEnroll {
  route = {
    ...super.getRoute(),
    // Enterprise version includes creating both plugin
    // and integration resources.
    component: () => <IntegrationEnroll />,
  };

  hasAccess(flags: FeatureFlags) {
    if (cfg.oss.hideInaccessibleFeatures) {
      return flags.enrollIntegrationsOrPlugins;
    }
    return true;
  }
}

class FeatureIntegrationStatus implements TeleportFeature {
  category = NavigationCategory.Management;

  parent = FeatureIntegrations;

  route = {
    title: 'Integration Status',
    path: cfg.oss.routes.integrationStatus,
    component: IntegrationStatus,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.plugins;
  }
}

// ****************************
// Other Features
// ****************************

class FeatureClusters extends OSS.FeatureClusters {
  route = {
    ...super.getRoute(),
    component: ClustersE,
  };
}

class FeatureAccount extends OSS.FeatureAccount {
  route = {
    title: 'Account Settings',
    path: cfg.oss.routes.account,
    component: AccountE,
  };
}

export class FeatureDeviceTrustWeb implements TeleportFeature {
  route = {
    title: 'Authorize Web Session',
    path: cfg.oss.routes.deviceTrustAuthorize,
    component: PassthroughPage,
  };

  hasAccess() {
    return true;
  }

  logoOnlyTopbar = true;
  hideNavigation = true;
}

export class FeatureSSOConfirm implements TeleportFeature {
  route = {
    title: 'SSO Confirm',
    path: cfg.routes.ssoConfirm,
    component: SSOConfirm,
  };

  hasAccess() {
    return true;
  }

  hideNavigation = true;
  logoOnlyTopbar = true;
}

class FeatureHelpAndSupport extends OSS.FeatureHelpAndSupport {
  route = {
    title: 'Help & Support',
    path: cfg.oss.routes.support,
    exact: true,
    component: SupportE,
  };
}

class FeatureUsersE extends OSS.FeatureUsers {
  route = {
    ...super.getRoute(),
    component: Users,
  };
}

class FeatureAccessGraph implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Permissions;
  sideNavCategory = SideNavigationCategory.Policy;

  route = {
    title: `Access Graph - ${NavTitle.AccessGraphDashboard}`,
    path: cfg.routes.accessGraph.dashboard,
    exact: true,
    component: AccessGraph,
  };

  hasAccess(flags: FeatureFlags) {
    return storageService.getAccessGraphEnabled() && flags.accessGraph;
  }

  navigationItem = {
    title: NavTitle.AccessGraphDashboard,
    icon: Layout,
    getLink() {
      return cfg.routes.accessGraph.dashboard;
    },
    exact: false,
    searchableTags: ['access graph', 'graph', 'tag', 'dashboard'],
  };
}

class FeatureAccessGraphBrowse extends FeatureAccessGraph {
  route = {
    title: `Access Graph - ${NavTitle.AccessGraphBrowse}`,
    path: cfg.routes.accessGraph.browse,
    exact: false,
    component: AccessGraph,
  };

  navigationItem = {
    title: NavTitle.AccessGraphBrowse,
    icon: Table,
    getLink() {
      return cfg.routes.accessGraph.browse;
    },
    exact: false,
    searchableTags: ['access graph', 'graph', 'tag', 'browse'],
  };
}

class FeatureAccessGraphCrownJewels extends FeatureAccessGraph {
  route = {
    title: `Access Graph - ${NavTitle.AccessGraphCrownJewels}`,
    path: cfg.routes.accessGraph.crownJewels,
    exact: false,
    component: AccessGraph,
  };

  navigationItem = {
    title: NavTitle.AccessGraphCrownJewels,
    icon: Crown,
    getLink() {
      return cfg.routes.accessGraph.crownJewels;
    },
    exact: false,
    searchableTags: ['access graph', 'graph', 'tag', 'crown jewels'],
  };
}

class FeatureAccessGraphGraphExplorer extends FeatureAccessGraph {
  route = {
    title: `Access Graph - ${NavTitle.AccessGraphGraphExplorer}`,
    path: cfg.routes.accessGraph.graphExplorer,
    exact: false,
    component: AccessGraph,
  };

  navigationItem = {
    title: NavTitle.AccessGraphGraphExplorer,
    icon: LineSegments,
    getLink() {
      return cfg.routes.accessGraph.graphExplorer;
    },
    exact: false,
    searchableTags: ['access graph', 'graph', 'tag', 'graph explorer'],
  };
}

class FeatureAccessGraphSQLEditor extends FeatureAccessGraph {
  route = {
    title: `Access Graph - ${NavTitle.AccessGraphSQLEditor}`,
    path: cfg.routes.accessGraph.sqlEditor,
    exact: false,
    component: AccessGraph,
  };

  navigationItem = {
    title: NavTitle.AccessGraphSQLEditor,
    icon: Code,
    getLink() {
      return cfg.routes.accessGraph.sqlEditor;
    },
    exact: false,
    searchableTags: ['access graph', 'graph', 'tag', 'sql editor'],
  };
}

class FeatureAccessGraphIntegrations extends FeatureAccessGraph {
  route = {
    title: `Access Graph - ${NavTitle.Integrations}`,
    path: cfg.routes.accessGraph.integrations,
    exact: false,
    component: AccessGraph,
  };

  hasAccess(flags: FeatureFlags) {
    return super.hasAccess(flags) && flags.accessGraphIntegrations;
  }

  navigationItem = {
    title: NavTitle.Integrations,
    icon: Plugs,
    getLink() {
      return cfg.routes.accessGraph.integrations;
    },
    exact: false,
    searchableTags: ['access graph', 'graph', 'tag', 'sql editor'],
  };
}

export function getEnterpriseFeatures(): TeleportFeature[] {
  return [
    // Resources
    new FeatureUnifiedResources(),

    // Legacy links in the navigation for Houston
    new FeatureDownloadCenter(),
    new FeatureSupport(),

    // AddNew
    new FeatureDiscoverE(),
    new FeatureIntegrationEnroll(),
    new OSS.FeatureAddBots(),
    new FeatureNewAccessList(),

    // - Access
    new FeatureUsersE(),
    new OSS.FeatureRoles(),
    new OSS.FeatureBots(),
    new OSS.FeatureJoinTokens(),
    new FeatureAuthConnectors(),
    new FeatureIntegrations(),
    new FeatureIntegrationStatus(),

    // - Permissions
    new FeatureClusters(),
    new OSS.FeatureTrust(),

    // - Identity
    new FeatureAccessRequests(),
    new FeatureNewAccessRequest(),
    new FeatureReviewAccessRequests(),
    new FeatureAccessListManagement(),
    new OSS.FeatureLocks(),
    new FeatureNewLock(),
    new FeatureDeviceTrust(),
    new FeatureAccessMonitoring(),

    // - Audit
    new OSS.FeatureAudit(),
    new OSS.FeatureSessions(),
    new OSS.FeatureRecordings(),

    // - Policy
    new FeatureAccessGraph(),
    new FeatureAccessGraphBrowse(),
    new FeatureAccessGraphCrownJewels(),
    new FeatureAccessGraphGraphExplorer(),
    new FeatureAccessGraphSQLEditor(),
    new FeatureAccessGraphIntegrations(),

    // Other
    new FeatureAccount(),
    new FeatureHelpAndSupport(),
    new FeatureUsageSummary(),
    new FeatureDeviceTrustWeb(),
    new FeatureSSOConfirm(),
  ];
}
