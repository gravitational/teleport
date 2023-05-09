import React from 'react';
import * as Icons from 'design/Icon';
import { Chart, Profile } from 'design/Icon';
import * as OSS from 'teleport/features';
import {
  ManagementSection,
  NavigationCategory,
} from 'teleport/Navigation/categories';
import {
  AccessRequestsIcon,
  DevicesIcon,
  DownloadsIcon,
  SupportIcon,
} from 'design/SVGIcon';

import cfg from 'e-teleport/config';
import {
  ReviewRequests,
  NewRequest,
  RequestFeatureLocked,
} from 'e-teleport/Workflow';

import { Downloads } from 'e-teleport/Downloads';

import type {
  LockedFeatures,
  FeatureFlags,
  TeleportFeature,
  TeleportFeatureNavigationItem,
  TeleportFeatureRoute,
} from 'teleport/types';

const AuthConnectors = React.lazy(
  () =>
    import(
      /* webpackChunkName: "e-auth-connectors" */ 'e-teleport/AuthConnectors'
    )
);
const AccountE = React.lazy(
  () => import(/* webpackChunkName: "e-account" */ 'e-teleport/Account')
);
const Integrations = React.lazy(
  () =>
    import(/* webpackChunkName: "e-integrations" */ 'e-teleport/Integrations')
);
const IntegrationEnroll = React.lazy(
  () =>
    import(
      /* webpackChunkName: "e-integration-enroll" */ 'e-teleport/Integrations/IntegrationEnroll'
    )
);
const SupportE = React.lazy(
  () => import(/* webpackChunkName: "e-support" */ 'e-teleport/Support')
);
const NewLock = React.lazy(
  () => import(/* webpackChunkName: "new-lock" */ 'e-teleport/NewLockV2')
);

const DeviceTrust = React.lazy(
  () => import(/* webpackChunkName: "e-devices" */ 'e-teleport/DeviceTrust')
);

const BillingSummaryE = React.lazy(
  () =>
    import(
      /* webpackChunkName: "e-billing-summary" */ 'e-teleport/Billing/Summary'
    )
);

const PaymentsInvoicesE = React.lazy(
  () =>
    import(
      /* webpackChunkName: "e-payments-invoices" */ 'e-teleport/Billing/PaymentsAndInvoices'
    )
);

const InvoiceSettingsE = React.lazy(
  () =>
    import(
      /* webpackChunkName: "e-invoice-settings" */ 'e-teleport/Billing/InvoiceSettings'
    )
);

// ****************************
// Resource Features
// ****************************

class FeatureAccessRequests implements TeleportFeature {
  route: TeleportFeatureRoute; // intentionally undefined
  category = NavigationCategory.Resources;
  navigationItem: TeleportFeatureNavigationItem = {
    title: 'Access Requests',
    icon: <AccessRequestsIcon />,
  };

  hasAccess() {
    return !cfg.oss.isDashboard;
  }
  // display an alternate page with a call to action
  // in case the feature is locked in the cluster
  isLocked(lockedFeatures: LockedFeatures) {
    return lockedFeatures.accessRequests;
  }

  lockedRoute = {
    title: 'Access Requests',
    path: cfg.routes.requestNew,
    component: RequestFeatureLocked,
  };

  lockedNavigationItem = {
    title: 'Access Requests',
    icon: <AccessRequestsIcon />,
    getLink(clusterId: string) {
      return cfg.getNewAccessRequestRoute(clusterId);
    },
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

class FeatureNewLock extends OSS.FeatureNewLock {
  route = {
    ...super.getRoute(),
    // Enterprise version allows resources access requests
    // and device trusts to be locked.
    component: NewLock,
  };
}

// ****************************
//  Billing Features
// ****************************

export class FeatureSummary implements TeleportFeature {
  category = NavigationCategory.Management;
  section = ManagementSection.Billing;

  route = {
    title: 'Summary',
    path: cfg.routes.billingSummary,
    component: BillingSummaryE,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.billing && cfg.oss.isUsageBasedBilling;
  }

  navigationItem = {
    title: 'Summary',
    icon: <Chart />,
    getLink(clusterId: string) {
      return cfg.getBillingSummaryRoute(clusterId);
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
    return flags.billing && cfg.oss.isUsageBasedBilling;
  }

  navigationItem = {
    title: 'Payments and Invoices',
    icon: <Profile />,
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
    return flags.billing && cfg.oss.isUsageBasedBilling;
  }

  navigationItem = {
    title: 'Invoice Settings',
    icon: <Profile />,
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

class FeatureIntegrations extends OSS.FeatureIntegrations {
  route = {
    ...super.getRoute(),
    // Enterprise version includes the enterprise only
    //  "plugin" resource along with the base
    // "integration" resource.
    component: () => <Integrations />,
  };

  hasAccess(flags: FeatureFlags) {
    return flags.plugins || flags.integrations;
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
    new FeatureIntegrationEnroll(),

    // - Activity
    new OSS.FeatureRecordings(),
    new OSS.FeatureAudit(),

    // - Billing
    new FeatureSummary(),
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
