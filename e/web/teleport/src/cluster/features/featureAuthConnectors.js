import * as Icons from 'design/Icon';
import { FeatureBase } from 'teleport/components/withFeature';
import AuthConnectors from 'e-teleport/cluster/components/AuthConnectors';
import cfg from 'e-teleport/config';

export function makeNavItem(to) {
  return {
    title: 'Auth Connectors',
    Icon: Icons.Lock,
    to,
  };
}

class FeatureAuthConnectors extends FeatureBase {
  constructor() {
    super();
    this.Component = AuthConnectors;
  }

  getRoute() {
    return {
      title: 'Auth. Connectors',
      path: cfg.routes.clusterAuthConnectors,
      component: this.Component,
    };
  }

  onload({ context }) {
    if (!context.isAuthConnectorEnabled()) {
      this.setDisabled();
      return;
    }

    const navItem = makeNavItem(cfg.getAuthConnectorsRoute());
    context.storeNav.addSideItem(navItem);
    this.setReady();
  }
}

export default FeatureAuthConnectors;
