import * as Icons from 'design/Icon';
import { FeatureBase } from 'teleport/components/withFeature';
import Ctx from 'teleport/teleportContext';
import Roles from 'e-teleport/cluster/components/Roles';
import cfg from 'e-teleport/config';

export default class FeatureRoles extends FeatureBase {
  Component = Roles;

  getRoute() {
    return {
      title: 'Roles',
      path: cfg.routes.clusterRoles,
      component: this.Component,
    };
  }

  onload(context: Ctx) {
    if (!context.isRolesEnabled() || cfg.isLeafCluster()) {
      this.setDisabled();
      return;
    }

    context.storeNav.addSideItem({
      title: 'Roles',
      Icon: Icons.ClipboardUser,
      to: cfg.getRolesRoute(),
    });
  }
}
