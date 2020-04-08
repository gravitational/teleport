import * as Icons from 'design/Icon';
import { FeatureBase } from 'teleport/components/withFeature';
import Ctx from 'teleport/teleportContext';
import AuthConnectors from 'e-teleport/cluster/components/AuthConnectors';
import cfg from 'e-teleport/config';

class FeatureAuthConnectors extends FeatureBase {
  Component = AuthConnectors;

  getRoute() {
    return {
      title: 'Auth. Connectors',
      path: cfg.routes.clusterAuthConnectors,
      component: this.Component,
    };
  }

  onload(context: Ctx) {
    if (!context.isAuthConnectorEnabled() || cfg.isLeafCluster()) {
      this.setDisabled();
      return;
    }

    context.storeNav.addSideItem({
      title: 'Auth Connectors',
      Icon: Icons.Lock,
      to: cfg.getAuthConnectorsRoute(),
    });

    this.setReady();
  }
}

export default FeatureAuthConnectors;
