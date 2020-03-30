import * as Icons from 'design/Icon';
import { FeatureBase } from 'teleport/components/withFeature';
import TrustedClusters from 'e-teleport/cluster/components/TrustedClusters';
import Ctx from 'teleport/teleportContext';
import cfg from 'e-teleport/config';

export default class FeatureTrustedClusters extends FeatureBase {
  Component = TrustedClusters;

  getRoute() {
    return {
      title: 'Trusted Clusters',
      path: cfg.routes.clusterTrustedClusters,
      component: this.Component,
    };
  }

  onload(context: Ctx) {
    if (!context.isTrustedClustersEnabled()) {
      this.setDisabled();
      return;
    }

    context.storeNav.addSideItem({
      title: 'Trusted Clusters',
      Icon: Icons.LanAlt,
      to: cfg.getTrustedClustersRoute(),
    });
  }
}
