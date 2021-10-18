import * as Icons from 'design/Icon';
import * as OSS from 'teleport/features';
import Ctx from 'teleport/teleportContext';
import cfg from 'e-teleport/config';
import Workflow from 'e-teleport/Workflow';
import AuthConnectors from 'e-teleport/AuthConnectors';
import Billing from 'e-teleport/Billing';

class FeatureAuthConnectors {
  getTopNavTitle() {
    return 'Team';
  }

  route = {
    title: 'Auth Connectors',
    path: cfg.oss.routes.sso,
    exact: false,
    component: AuthConnectors,
  };

  register(ctx: Ctx) {
    if (!ctx.getFeatureFlags().authConnector) {
      return;
    }

    ctx.storeNav.addSideItem({
      group: 'team',
      title: 'Auth Connectors',
      Icon: Icons.Lock,
      exact: false,
      getLink() {
        return cfg.oss.routes.sso;
      },
    });

    ctx.features.push(this);
  }
}

class FeatureWorkflow {
  getTopNavTitle() {
    return 'Activity';
  }

  route = {
    group: 'activity',
    title: 'Access Requests',
    path: cfg.getAccessRequestRoute(),
    component: Workflow,
  };

  register(ctx: Ctx) {
    ctx.storeNav.addSideItem({
      group: 'activity',
      title: 'Access Requests',
      Icon: Icons.EqualizerVertical,
      getLink() {
        return cfg.getAccessRequestRoute();
      },
    });

    ctx.features.push(this);
  }
}

class FeatureBilling {
  getTopNavTitle() {
    return 'Billing & Usage';
  }

  route = {
    title: 'Billing & Usage',
    path: cfg.routes.billing,
    component: Billing,
  };

  register(ctx: Ctx) {
    if (!ctx.getFeatureFlags().billing) {
      return;
    }

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

export default function getFeatures() {
  return [
    new OSS.FeatureNodes(),
    new OSS.FeatureApps(),
    new OSS.FeatureKubes(),
    new OSS.FeatureDatabases(),
    new OSS.FeatureDesktops(),
    new OSS.FeatureSessions(),
    new OSS.FeatureRecordings(),
    new OSS.FeatureAudit(),
    new OSS.FeatureUsers(),
    new OSS.FeatureRoles(),
    new FeatureAuthConnectors(),
    new OSS.FeatureClusters(),
    new OSS.FeatureTrust(),
    new OSS.FeatureHelpAndSupport(),
    new OSS.FeatureAccount(),
    new FeatureWorkflow(),
    new FeatureBilling(),
  ];
}
