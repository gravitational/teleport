import * as Icons from 'design/Icon';
import * as Features from 'teleport/features';
import Ctx from 'teleport/teleportContext';
import cfgE from 'e-teleport/config';
import cfg from 'teleport/config';
import Workflow from 'e-teleport/Workflow';
import AuthConnectors from 'e-teleport/AuthConnectors';

class FeatureAuthConnectors {
  getTopNavTitle() {
    return 'Team';
  }

  route = {
    title: 'Auth Connectors',
    path: cfg.routes.sso,
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
        return cfg.routes.sso;
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
    path: cfgE.routes.requests,
    component: Workflow,
  };

  register(ctx: Ctx) {
    ctx.storeNav.addSideItem({
      group: 'activity',
      title: 'Access Requests',
      Icon: Icons.EqualizerVertical,
      getLink() {
        return cfgE.routes.requests;
      },
    });

    ctx.features.push(this);
  }
}

export default function getFeatures() {
  return [
    new Features.FeatureNodes(),
    new Features.FeatureApps(),
    new Features.FeatureSessions(),
    new Features.FeatureRecordings(),
    new Features.FeatureAudit(),
    new Features.FeatureUsers(),
    new Features.FeatureRoles(),
    new FeatureAuthConnectors(),
    new Features.FeatureClusters(),
    new Features.FeatureTrust(),
    new Features.FeatureHelpAndSupport(),
    new Features.FeatureAccount(),
    new FeatureWorkflow(),
  ];
}
