import withFeature, { FeatureBase } from 'gravity/components/withFeature';
import { addSideNavItem } from 'gravity/cluster/flux/nav/actions';
import * as Icons from 'design/Icon';
import cfg from 'e-gravity/config';
import { fetchRoles } from 'e-gravity/cluster/flux/roles/actions';
import Roles from './../components/Roles';

export const makeNavItem = to => ({
  title: 'Roles',
  Icon: Icons.ClipboardUser,
  to
})

class FeatureRoles extends FeatureBase {

  constructor() {
    super()
    this.Component = withFeature(this)(Roles);
  }

  getRoute(){
    return {
      title: 'Roles',
      path: cfg.routes.clusterRoles,
      exact: true,
      component: this.Component
    }
  }

  onload(context) {
    const allowed = context.featureFlags.clusterRoles()
    if (!allowed) {
      this.setDisabled();
      return;
    }

    const navItem = makeNavItem(cfg.getClusterRolesRoute());
    addSideNavItem(navItem);

    this.setProcessing();
    fetchRoles()
      .done(this.setReady.bind(this))
      .fail(this.setFailed.bind(this));
  }
}

export default FeatureRoles;