import * as Icons from 'design/Icon';
import { FeatureBase } from 'teleport/components/withFeature';
import TrustedClusters from 'e-teleport/cluster/components/TrustedClusters';
import cfg from 'e-teleport/config';

export function makeNavItem(to) {
  return {
    title: 'Trusted Clusters',
    Icon: Icons.Link,
    to,
  };
}

export default class FeatureTrustedClusters extends FeatureBase {
  constructor() {
    super();
    this.Component = TrustedClusters;
  }

  getRoute() {
    return {
      title: 'Trusted Clusters',
      path: cfg.routes.clusterTrustedClusters,
      component: this.Component,
    };
  }

  onload({ context }) {
    if (!context.isTrustedClustersEnabled()) {
      this.setDisabled();
      return;
    }

    const navItem = makeNavItem(cfg.getTrustedClustersRoute());
    context.storeNav.addSideItem(navItem);
  }
}
