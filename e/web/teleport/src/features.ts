import * as Icons from 'design/Icon';
import * as Features from 'teleport/features';
import Ctx from 'teleport/teleportContext';
import cfg from 'e-teleport/config';
import Workflow from 'e-teleport/Workflow';

class FeatureWorkflow {
  getTopNavTitle() {
    return 'Activity';
  }

  route = {
    group: 'activity',
    title: 'Access Requests',
    path: cfg.routes.requests,
    component: Workflow,
  };

  register(ctx: Ctx) {
    if (!ctx.getFeatureFlags().workflow) {
      return;
    }

    ctx.storeNav.addSideItem({
      group: 'activity',
      title: 'Access Requests',
      Icon: Icons.EqualizerVertical,
      getLink() {
        return cfg.routes.requests;
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
    new Features.FeatureAuthConnectors(),
    new Features.FeatureClusters(),
    new Features.FeatureTrust(),
    new Features.FeatureHelpAndSupport(),
    new Features.FeatureAccount(),
    new FeatureWorkflow(),
  ];
}
