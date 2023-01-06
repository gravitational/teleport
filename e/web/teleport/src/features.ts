import React from 'react';

import * as Icons from 'design/Icon';
import * as OSS from 'teleport/features';
import { Feature } from 'teleport/types';
import Ctx from 'teleport/teleportContext';

import cfg from 'e-teleport/config';
import { ReviewRequests, NewRequest } from 'e-teleport/Workflow';

const AuthConnectors = React.lazy(
  () =>
    import(
      /* webpackChunkName: "e-auth-connectors" */ 'e-teleport/AuthConnectors'
    )
);
const AccountE = React.lazy(
  () => import(/* webpackChunkName: "e-account" */ 'e-teleport/Account')
);
const SupportE = React.lazy(
  () => import(/* webpackChunkName: "e-support" */ 'e-teleport/Support')
);

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

  isAvailable(ctx: Ctx): boolean {
    return ctx.getFeatureFlags().accessRequests;
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

  isAvailable(ctx: Ctx): boolean {
    return ctx.getFeatureFlags().newAccessRequest;
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

export function getEnterpriseFeatures() {
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
    new FeatureHelpAndSupport(),
    new FeatureAccount(),
  ];
}
