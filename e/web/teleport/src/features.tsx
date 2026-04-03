import { lazy } from 'react';

import {
  Add,
  Chart,
  Code,
  Crown,
  Detective,
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
  Warning,
  XCheck,
} from 'design/Icon';

import { AccessListManagement } from 'e-teleport/AccessListManagement';
import { CreateAccessListWithProvider } from 'e-teleport/AccessListManagement/CreateAccessList';
import { AccessMonitoring } from 'e-teleport/AccessMonitoring';
import { Account as AccountE } from 'e-teleport/Account';
import { AuthConnectors } from 'e-teleport/AuthConnectors';
import { PassthroughPage } from 'e-teleport/AuthorizeDeviceWeb/AuthorizeDeviceWeb';
import cfg from 'e-teleport/config';
import { DeviceTrust } from 'e-teleport/DeviceTrust';
import { Discover as DiscoverE } from 'e-teleport/Discover';
import { Downloads } from 'e-teleport/Downloads';
import Integrations from 'e-teleport/Integrations';
import { IntegrationEnroll } from 'e-teleport/Integrations/IntegrationEnroll';
import { IntegrationStatus } from 'e-teleport/Integrations/IntegrationStatus';
import { NewLock } from 'e-teleport/NewLockV2';
import { ListSessionRecordingsRouteE } from 'e-teleport/SessionRecordings/list/ListSessionRecordingsRouteE';
import { SessionSummaries } from 'e-teleport/SessionRecordings/setup/SessionSummaries';
import { SSOConfirm } from 'e-teleport/SSOConfirm/SSOConfirm';
import SupportE from 'e-teleport/Support';
import { UnifiedResourcesE } from 'e-teleport/UnifiedResources';
import { Users } from 'e-teleport/Users';
import NewRequest from 'e-teleport/Workflow/NewRequest/NewRequest';
import ReviewRequests from 'e-teleport/Workflow/ReviewRequests/ReviewRequests';
import { Redirect } from 'teleport/components/Router';
import * as OSS from 'teleport/features';
import { NavigationCategory } from 'teleport/Navigation/categories';
import { storageService } from 'teleport/services/storageService';
import {
  NavTitle,
  type FeatureFlags,
  type TeleportFeature,
} from 'teleport/types';

import { AccessAutomations } from './AccessAutomations/AccessAutomations';
import { RolesE } from './Roles/RolesE';

// ****************************
// Resource Features
// ****************************

const AccessGraph = lazy(() => import('e-teleport/AccessGraph'));
const Cloud = lazy(() => import('e-teleport/Cloud'));

class FeatureUnifiedResources extends OSS.FeatureUnifiedResources {
  route = {
    ...super.getRoute(),
    // Enterprise Unified Resources can display requestable resources
    // and allows the creation of access requests
    component: UnifiedResourcesE,
  };
}

class FeatureRoles extends OSS.FeatureRoles {
  route = {
    ...super.getRoute(),
    component: RolesE,
  };
}

class FeatureNewAccessRequest implements TeleportFeature {
  category = NavigationCategory.IdentityGovernance;

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

class FeatureAccessRequests implements TeleportFeature {
  category = NavigationCategory.IdentityGovernance;

  route = {
    title: 'Access Requests',
    path: cfg.routes.requests,
    component: ReviewRequests,
  };

  hasAccess() {
    return !cfg.oss.isDashboard;
  }

  navigationItem = {
    title: NavTitle.AccessRequests,
    icon: ListAddCheck,
    getLink() {
      return cfg.getAccessRequestRoute();
    },
    searchableTags: ['access requests', 'requests', 'review'],
  };
}

class FeatureAccessAutomations implements TeleportFeature {
  category = NavigationCategory.IdentityGovernance;

  route = {
    title: 'Access Automations',
    path: cfg.routes.accessAutomations,
    component: AccessAutomations,
  };

  hasAccess() {
    return !cfg.oss.isDashboard;
  }

  navigationItem = {
    title: NavTitle.AccessAutomations,
    icon: XCheck,
    getLink() {
      return cfg.getAccessAutomationRoute();
    },
    searchableTags: ['access automations', 'automations'],
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

// FeatureLegacyUsageSummary redirects the old per-cluster usage summary route
// (/web/cluster/:clusterId/usage-summary) to the new cluster-agnostic cloud
// route. TODO(@mcbattirola): Remove in v20.
class FeatureLegacyUsageSummary implements TeleportFeature {
  route = {
    title: 'Usage Tracking',
    path: cfg.routes.usageSummarySummary,
    component: () => <Redirect to={cfg.routes.cloud.usageSummary} />,
  };

  hasAccess(flags: FeatureFlags) {
    return (
      flags.billing && cfg.oss.isUsageBasedBilling && !cfg.oss.isStripeManaged
    );
  }
}

export class FeatureUsageSummary implements TeleportFeature {
  route = {
    title: 'Usage Tracking',
    // Registered at the root so the Cloud component handles all sub-routes
    // (/web/cloud/summary, /web/cloud/mau, /web/cloud/tpr) internally via
    // React Router, without needing separate feature registrations.
    path: cfg.routes.cloud.root,
    component: Cloud,
  };

  hasAccess(flags: FeatureFlags) {
    return (
      flags.billing && cfg.oss.isUsageBasedBilling && !cfg.oss.isStripeManaged
    );
  }

  topMenuItem = {
    title: NavTitle.UsageReporting,
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

  showInDashboard = true;
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
  category = NavigationCategory.IdentityGovernance;

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
  category = NavigationCategory.IdentityGovernance;

  route = {
    title: 'Manage Access Lists',
    path: cfg.routes.accessListsList,
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
  category = NavigationCategory.AddNew;

  route = {
    title: NavTitle.NewAccessList,
    path: cfg.routes.accessListNew,
    exact: true,
    component: CreateAccessListWithProvider,
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
  category = NavigationCategory.ZeroTrustAccess;

  route = {
    title: 'Trusted Devices',
    path: cfg.routes.deviceTrust,
    exact: true,
    component: DeviceTrust,
  };

  hasAccess(flags: FeatureFlags) {
    if (OSS.shouldHideFromNavigation(cfg.oss)) {
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
  route = {
    ...super.getRoute(),
    // Enterprise version includes the enterprise only
    //  "plugin" resource along with the base
    // "integration" resource.
    component: Integrations,
  };

  hasAccess(flags: FeatureFlags) {
    // if feature hiding is enabled, only show
    // if the user has access
    if (OSS.shouldHideFromNavigation(cfg.oss)) {
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
    component: IntegrationEnroll,
  };

  hasAccess(flags: FeatureFlags) {
    if (OSS.shouldHideFromNavigation(cfg.oss)) {
      return flags.enrollIntegrationsOrPlugins;
    }
    return true;
  }
}

class FeatureIntegrationStatus implements TeleportFeature {
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
  category = NavigationCategory.IdentitySecurity;

  route = {
    title: `Access Graph - ${NavTitle.AccessGraphDashboard}`,
    path: cfg.routes.accessGraph.dashboard,
    exact: true,
    component: AccessGraph,
  };

  hasAccess(flags: FeatureFlags) {
    if (
      OSS.shouldHideFromNavigation(cfg.oss) ||
      this.route.path !== cfg.routes.accessGraph.dashboard
    ) {
      return storageService.getAccessGraphEnabled() && flags.accessGraph;
    }
    return true;
  }

  navigationItem = {
    title: NavTitle.AccessGraphDashboard,
    icon: Layout,
    getLink() {
      return cfg.routes.accessGraph.dashboard;
    },
    exact: false,
    searchableTags: [
      'access graph',
      'graph',
      'tag',
      'dashboard',
      'identity security',
    ],
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
    searchableTags: [
      'access graph',
      'graph',
      'tag',
      'browse',
      'identity security',
    ],
  };
}

class FeatureAccessGraphAlerts extends FeatureAccessGraph {
  route = {
    title: `Access Graph - ${NavTitle.AccessGraphAlerts}`,
    path: cfg.routes.accessGraph.alerts,
    exact: false,
    component: AccessGraph,
  };

  navigationItem = {
    title: NavTitle.AccessGraphAlerts,
    icon: Warning,
    getLink() {
      return cfg.routes.accessGraph.alerts;
    },
    exact: false,
    searchableTags: [
      'access graph',
      'graph',
      'tag',
      'alerts',
      'identity security',
    ],
  };
}

class FeatureAccessGraphInvestigate extends FeatureAccessGraph {
  route = {
    title: `Access Graph - ${NavTitle.AccessGraphInvestigate}`,
    path: cfg.routes.accessGraph.investigate,
    exact: false,
    component: AccessGraph,
  };

  navigationItem = {
    title: NavTitle.AccessGraphInvestigate,
    icon: Detective,
    getLink() {
      return cfg.routes.accessGraph.investigate;
    },
    exact: false,
    searchableTags: [
      'access graph',
      'graph',
      'tag',
      'investigate',
      'identity security',
    ],
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
    searchableTags: [
      'access graph',
      'graph',
      'tag',
      'crown jewels',
      'identity security',
    ],
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
    searchableTags: [
      'access graph',
      'graph',
      'tag',
      'graph explorer',
      'identity security',
    ],
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
    searchableTags: [
      'access graph',
      'graph',
      'tag',
      'sql editor',
      'identity security',
    ],
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
    return (
      storageService.getAccessGraphEnabled() &&
      flags.accessGraph &&
      flags.accessGraphIntegrations
    );
  }

  navigationItem = {
    title: NavTitle.Integrations,
    icon: Plugs,
    getLink() {
      return cfg.routes.accessGraph.integrations;
    },
    exact: false,
    searchableTags: [
      'access graph',
      'graph',
      'tag',
      'sql editor',
      'identity security',
    ],
  };
}

class FeatureRecordings extends OSS.FeatureRecordings {
  route = {
    title: 'Session Recordings',
    path: cfg.oss.routes.recordings,
    exact: true,
    component: ListSessionRecordingsRouteE,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.recordings;
  }
}

class FeatureSessionSummaries implements TeleportFeature {
  parent = FeatureRecordings;

  route = {
    title: 'Session Summaries',
    path: cfg.routes.sessionSummariesManagement,
    component: SessionSummaries,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.sessionSummaries;
  }
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
    new FeatureRoles(),
    new FeatureDeviceTrust(),
    new OSS.FeatureBots(),
    new OSS.FeatureBotDetails(),
    new OSS.FeatureBotInstances(),
    new OSS.FeatureInstances(),
    new OSS.FeatureManagedUpdates(),
    new OSS.FeatureBotInstanceDetails(),
    new OSS.FeatureWorkloadIdentity(),
    new OSS.FeatureAddBotsShortcut(),
    new OSS.FeatureJoinTokens(),
    new FeatureAuthConnectors(),
    new FeatureIntegrations(),
    new FeatureIntegrationStatus(),
    new OSS.FeatureIntegrationOverview(),

    // - Permissions
    new OSS.FeatureClusters(),
    new OSS.FeatureTrust(),

    // - Identity
    new FeatureAccessRequests(),
    new FeatureNewAccessRequest(),
    new FeatureAccessListManagement(),
    new FeatureAccessAutomations(),
    new OSS.FeatureLocks(),
    new FeatureNewLock(),
    new FeatureAccessMonitoring(),

    // - Audit
    new OSS.FeatureAudit(),
    new OSS.FeatureSessions(),
    new FeatureRecordings(),
    new FeatureSessionSummaries(),

    // - Policy
    new FeatureAccessGraph(),
    new FeatureAccessGraphBrowse(),
    new FeatureAccessGraphAlerts(),
    new FeatureAccessGraphInvestigate(),
    new FeatureAccessGraphCrownJewels(),
    new FeatureAccessGraphGraphExplorer(),
    new FeatureAccessGraphSQLEditor(),
    new FeatureAccessGraphIntegrations(),

    // Other
    new FeatureAccount(),
    new FeatureHelpAndSupport(),
    new FeatureLegacyUsageSummary(),
    new FeatureUsageSummary(),
    new FeatureDeviceTrustWeb(),
    new FeatureSSOConfirm(),
  ];
}
