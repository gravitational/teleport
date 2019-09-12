import cfg from 'e-gravity/config'
import { addUserRoleNavItem } from 'e-gravity/hub/flux/nav/actions';
import FeatureUsers, { makeNavItem } from 'e-gravity/cluster/features/featureUsers';

class FeatureHubRoles extends FeatureUsers {

  getRoute(){
    return {
      ...super.getRoute(),
      path: cfg.routes.hubAccessUsers
    }
  }

  onload(context) {
    super.onload(context);
    if(!this.isDisabled()){
      const item = makeNavItem(cfg.routes.hubAccessUsers);
      addUserRoleNavItem(item);
    }
  }
}

export default FeatureHubRoles;