import React, { lazy } from 'react';
import * as OSS from 'teleport/features';
import {
  ManagementSection,
  NavigationCategory,
} from 'teleport/Navigation/categories';

import {
  Add,
  Chart,
  Download,
  FlowArrow,
  Headset,
  Invoices,
  Laptop,
  ListAddCheck,
  Profile,
  UserList,
  Graph,
} from 'design/Icon';

import { NavTitle } from 'teleport/types';

import { storageService } from 'teleport/services/storageService';

import cfg from 'e-teleport/config';

import { NewRequest, ReviewRequests } from 'e-teleport/Workflow';

import { Downloads } from 'e-teleport/Downloads';

import type {
  FeatureFlags,
  TeleportFeature,
  TeleportFeatureNavigationItem,
  TeleportFeatureRoute,
} from 'teleport/types';

const AuthConnectors = lazy(() => import('e-teleport/AuthConnectors'));
const AccountE = lazy(() => import('e-teleport/Account/AccountNew'));
const Integrations = lazy(() => import('e-teleport/Integrations'));
const IntegrationEnroll = lazy(
  () => import('e-teleport/Integrations/IntegrationEnroll')
);
const SupportE = lazy(() => import('e-teleport/Support'));
const NewLock = lazy(() => import('e-teleport/NewLockV2'));

const DeviceTrust = lazy(() => import('e-teleport/DeviceTrust'));

const BillingSummaryE = lazy(() => import('e-teleport/Billing/Summary'));
const EubpBillingSummaryE = lazy(
  () => import('e-teleport/Billing/EubpSummary')
);

const PaymentsInvoicesE = lazy(
  () => import('e-teleport/Billing/PaymentsAndInvoices')
);

const InvoiceSettingsE = lazy(
  () => import('e-teleport/Billing/InvoiceSettings')
);

const DiscoverE = lazy(() => import('e-teleport/Discover'));
const AccessListManagement = lazy(
  () => import('e-teleport/AccessListManagement')
);
const AccessMonitoring = lazy(() => import('e-teleport/AccessMonitoring'));
const AccessGraph = lazy(() => import('e-teleport/AccessGraph'));

const Users = lazy(() => import('teleport/Users'));

const InviteCollaboratorsDialog = lazy(
  () => import('e-teleport/InviteCollaborators')
);
const EmailPasswordResetDialog = lazy(
  () => import('e-teleport/InviteCollaborators/EmailPasswordResetDialog')
);

// ****************************
// Resource Features
// ****************************

class FeatureAccessRequests implements TeleportFeature {
  route: TeleportFeatureRoute; // intentionally undefined
  category = NavigationCategory.Resources;
  navigationItem: TeleportFeatureNavigationItem = {
    title: NavTitle.AccessRequests,
    icon: ListAddCheck,
    getLink() {
      return cfg.getAccessRequestRoute();
    },
  };

  hasAccess(flags: FeatureFlags) {
    return flags.accessRequests;
  }
  topMenuItem = this.navigationItem;
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
    title: NavTitle.NewRequest,
    icon: Add,
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

export class FeatureTeamSummary implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Billing;

  route = {
    title: 'Summary',
    path: cfg.routes.billingSummary,
    component: BillingSummaryE,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.billing && cfg.oss.isTeam;
  }

  navigationItem = {
    title: NavTitle.BillingSummary,
    icon: Chart,
    getLink(clusterId: string) {
      return cfg.getBillingSummaryRoute(clusterId);
    },
  };
}

export class FeatureEubpSummary implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Billing;

  route = {
    title: 'Usage Tracking',
    path: cfg.routes.eubpBillingSummary,
    component: EubpBillingSummaryE,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.billing && cfg.oss.isUsageBasedBilling && !cfg.oss.isTeam;
  }

  navigationItem = {
    title: NavTitle.BillingSummary,
    icon: Chart,
    getLink(clusterId: string) {
      return cfg.getEubpBillingSummaryRoute(clusterId);
    },
  };
}

export class FeaturePaymentsAndInvoices implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Billing;

  route = {
    title: 'Payments and Invoices',
    path: cfg.routes.paymentsInvoices,
    component: PaymentsInvoicesE,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.billing && cfg.oss.isTeam;
  }

  navigationItem = {
    title: NavTitle.PaymentsAndInvoices,
    icon: Invoices,
    getLink(clusterId: string) {
      return cfg.getPaymentsInvoicesRoute(clusterId);
    },
  };
}

export class FeatureInvoiceSettings implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Billing;

  route = {
    title: 'Invoice Settings',
    path: cfg.routes.invoiceSettings,
    component: InvoiceSettingsE,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.billing && cfg.oss.isTeam;
  }

  navigationItem = {
    title: NavTitle.InvoiceSettings,
    icon: Profile,
    getLink(clusterId: string) {
      return cfg.getInvoiceSettingsRoute(clusterId);
    },
  };
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
    title: NavTitle.Downloads,
    icon: Download,
    getLink() {
      return cfg.routes.downloadCenter;
    },
  };
  topMenuItem = this.navigationItem;
}

class FeatureSupport implements TeleportFeature {
  category = NavigationCategory.Resources;

  hasAccess(flags: FeatureFlags) {
    return flags.downloadCenter; // use the same flag as the download center to hide for non-Houston deployments
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
  };
}

class FeatureDeviceTrust implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Identity;
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
    title: NavTitle.TrustedDevices,
    icon: Laptop,
    exact: true,
    getLink() {
      return cfg.routes.deviceTrust;
    },
  };
}

class FeatureIntegrations extends OSS.FeatureIntegrations {
  route = {
    ...super.getRoute(),
    // Enterprise version includes the enterprise only
    //  "plugin" resource along with the base
    // "integration" resource.
    component: () => <Integrations />,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.plugins || flags.integrations || flags.externalAuditStorage;
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
    return flags.enrollIntegrationsOrPlugins;
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
    component: () => (
      <Users
        InviteCollaborators={cfg.oss.isCloud ? InviteCollaboratorsDialog : null}
        EmailPasswordReset={cfg.oss.isCloud ? EmailPasswordResetDialog : null}
      />
    ),
  };
}

class FeatureAccessGraph implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Permissions;

  hideNavigation = true;

  route = {
    title: 'Access Graph',
    path: cfg.routes.accessGraph,
    exact: true,
    component: AccessGraph,
  };

  hasAccess(flags: FeatureFlags) {
    return storageService.getAccessGraphEnabled() && flags.accessGraph;
  }

  navigationItem = {
    title: NavTitle.AccessGraph,
    icon: FlowArrow,
    getLink() {
      return cfg.routes.accessGraph;
    },
  };
}

export function getEnterpriseFeatures(): TeleportFeature[] {
  return [
    // Resources
    new OSS.FeatureUnifiedResources(),
    new FeatureAccessRequests(),
    new FeatureNewAccessRequest(),
    new FeatureReviewAccessRequests(),
    new OSS.FeatureSessions(),

    // Legacy links in the navigation for Houston
    new FeatureDownloadCenter(),
    new FeatureSupport(),

    // Management

    // - Access
    new FeatureUsersE(),
    new FeatureAuthConnectors(),
    new FeatureIntegrations(),
    new FeatureDiscoverE(),
    new FeatureIntegrationEnroll(),

    // - Permissions
    new OSS.FeatureRoles(),
    new FeatureAccessGraph(),

    // - Identity
    new FeatureAccessListManagement(),
    new OSS.FeatureLocks(),
    new FeatureNewLock(),
    new FeatureDeviceTrust(),
    new FeatureAccessMonitoring(),

    // - Activity
    new OSS.FeatureRecordings(),
    new OSS.FeatureAudit(),

    // - Billing
    new FeatureTeamSummary(),
    new FeatureEubpSummary(),
    new FeaturePaymentsAndInvoices(),
    new FeatureInvoiceSettings(),

    // - Clusters
    new OSS.FeatureClusters(),
    new OSS.FeatureTrust(),

    // Other
    new FeatureAccount(),
    new FeatureHelpAndSupport(),
  ];
}
