import { SettingsFeatureBase } from 'telebase-app/features/settings/featureSettings';
import { addNavItem } from 'telebase-app/flux/settings/actions';
import { withDocTitle } from 'telebase-app/components/documentTitle';

import cfg from './../config'
import { fetchRoles } from './../flux/settingsRoles/actions';
import SettingsRoles from './../components/settings/tabRoles'
import * as flags from './featureFlags';

class RolesFeature extends SettingsFeatureBase {

  constructor(routes) {
    super();
    const route = {
      path: cfg.routes.settingsRoles,
      component: super.withMe(withDocTitle('Roles', SettingsRoles))
    };

    routes.push(route);
  }

  componentDidMount() {
    this.init()
  }

  init(){
    if (this.wasInitialized()) {
      return;
    }

    this.startProcessing();
    fetchRoles()
      .done(this.stopProcessing.bind(this))
      .fail(this.handleError.bind(this))
  }

  isEnabled(){
    return flags.isRolesEnabled();
  }

  onload() {
    const navItem = {
      to: cfg.routes.settingsRoles,
      title: "Roles"
    }

    if (this.isEnabled()) {
      addNavItem(navItem);
    }
  }
}

export default RolesFeature;