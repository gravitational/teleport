import cfg from 'e-gravity/config'
import { addUserRoleNavItem } from 'e-gravity/hub/flux/nav/actions';
import FeatureAuthConnectors, { makeNavItem } from 'e-gravity/cluster/features/featureAuthConnectors';

class FeatureHubConnectors extends FeatureAuthConnectors {

  getRoute(){
    return {
      ...super.getRoute(),
      path: cfg.routes.hubAccessAuth
    }
  }

  onload(context) {
    super.onload(context);
    if(!this.isDisabled()){
      const item = makeNavItem(cfg.routes.hubAccessAuth);
      addUserRoleNavItem(item);
    }
  }
}

export default FeatureHubConnectors;