import * as Icons from 'design/Icon';
import { FeatureBase } from 'teleport/components/withFeature';
import Roles from 'e-teleport/cluster/components/Roles';
import cfg from 'e-teleport/config';

export function makeNavItem(to) {
  return {
    title: 'Roles',
    Icon: Icons.ClipboardUser,
    to,
  };
}

export default class FeatureRoles extends FeatureBase {
  constructor() {
    super();
    this.Component = Roles;
  }

  getRoute() {
    return {
      title: 'Roles',
      path: cfg.routes.clusterRoles,
      component: this.Component,
    };
  }

  onload({ context }) {
    if (!context.isRolesEnabled()) {
      this.setDisabled();
      return;
    }

    const navItem = makeNavItem(cfg.getRolesRoute());
    context.storeNav.addSideItem(navItem);
  }
}
