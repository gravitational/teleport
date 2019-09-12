import withFeature, { FeatureBase } from 'gravity/components/withFeature';
import { addSideNavItem } from 'gravity/cluster/flux/nav/actions';
import * as Icons from 'design/Icon';
import AuthConnectors from 'e-gravity/cluster/components/AuthConnectors';
import { fetchAuthProviders } from 'e-gravity/cluster/flux/authConnectors/actions';
import cfg from 'e-gravity/config';

export function makeNavItem(to) {
  return {
    title: 'Auth Connectors',
    Icon: Icons.Lock,
    to
  }
}

class FeatureAuthConnectors extends FeatureBase {

  constructor() {
    super()
    this.Component = withFeature(this)(AuthConnectors);
  }

  getRoute(){
    return {
      title: 'Auth. Connectors',
      path: cfg.routes.clusterAuthConnectors,
      component: this.Component
    }
  }

  onload({featureFlags}) {
    const allowed = featureFlags.clusterAuthConnectors();
    if (!allowed) {
      this.setDisabled();
      return;
    }

    const navItem = makeNavItem(cfg.getClusterAuthConnectorsRoute());
    addSideNavItem(navItem);

    this.setProcessing();
    fetchAuthProviders()
      .done(this.setReady.bind(this))
      .fail(this.setFailed.bind(this));
  }
}

export default FeatureAuthConnectors;