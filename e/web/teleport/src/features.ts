import * as Icons from 'design/Icon';
import * as OSS from 'teleport/features';
import { Feature } from 'teleport/types';
import Ctx from 'teleport/teleportContext';
import cfg from 'e-teleport/config';
import { ReviewRequests, NewRequest } from 'e-teleport/Workflow';
import AuthConnectors from 'e-teleport/AuthConnectors';
import Billing from 'e-teleport/Billing';
import AccountE from 'e-teleport/Account';

class FeatureAuthConnectors extends OSS.FeatureAuthConnectors {
  route = {
    title: 'Auth Connectors',
    path: cfg.oss.routes.sso,
    exact: false,
    component: AuthConnectors,
  };
}

class FeatureReviewAccessRequests extends Feature {
  topNavTitle = 'Access Requests';

  route = {
    title: 'Review Requests',
    path: cfg.routes.requests,
    component: ReviewRequests,
  };

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  isAvailable(ctx: Ctx): boolean {
    return true; // TODO(isaiah)
  }

  register(ctx: Ctx) {
    ctx.storeNav.addSideItem({
      group: 'accessrequests',
      title: 'Review Requests',
      Icon: Icons.ListAddCheck,
      getLink() {
        return cfg.getAccessRequestRoute();
      },
    });

    ctx.features.push(this);
  }
}

class FeatureNewAccessRequest extends Feature {
  topNavTitle = '';

  route = {
    title: 'New Request',
    path: cfg.routes.requestNew,
    component: NewRequest,
  };

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  isAvailable(ctx: Ctx): boolean {
    return true; // TODO(isaiah)
  }

  register(ctx: Ctx) {
    ctx.storeNav.addSideItem({
      group: 'accessrequests',
      title: 'New Request',
      Icon: Icons.Add,
      getLink(clusterId: string) {
        return cfg.getNewAccessRequestRoute(clusterId);
      },
    });

    ctx.features.push(this);
  }
}

class FeatureBilling extends Feature {
  topNavTitle = 'Billing & Usage';

  route = {
    title: 'Billing & Usage',
    path: cfg.routes.billing,
    component: Billing,
  };

  isAvailable(ctx: Ctx): boolean {
    return ctx.getFeatureFlags().billing;
  }

  register(ctx: Ctx) {
    ctx.storeNav.addTopMenuItem({
      title: 'Billing & Usage',
      Icon: Icons.CreditCard,
      getLink() {
        return cfg.routes.billingUsage;
      },
    });

    ctx.features.push(this);
  }
}

class FeatureAccount extends OSS.FeatureAccount {
  route = {
    title: 'Account Settings',
    path: cfg.oss.routes.account,
    component: AccountE,
  };
}

export default function getFeatures() {
  return [
    new OSS.FeatureNodes(),
    new OSS.FeatureApps(),
    new OSS.FeatureKubes(),
    new OSS.FeatureDatabases(),
    new OSS.FeatureDesktops(),
    new FeatureNewAccessRequest(),
    new FeatureReviewAccessRequests(),
    new OSS.FeatureSessions(),
    new OSS.FeatureRecordings(),
    new OSS.FeatureAudit(),
    new OSS.FeatureUsers(),
    new OSS.FeatureRoles(),
    new FeatureAuthConnectors(),
    new OSS.FeatureClusters(),
    new OSS.FeatureTrust(),
    new OSS.FeatureHelpAndSupport(),
    new FeatureAccount(),
    new FeatureBilling(),
  ];
}
