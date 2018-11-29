import { SettingsFeatureBase } from 'telebase-app/features/settings/featureSettings';
import { addNavItem } from 'telebase-app/flux/settings/actions';
import { withDocTitle } from 'telebase-app/components/documentTitle';

import cfg from '../config'
import { fetchAuthProviders } from './../flux/settingsAuth/actions';
import SettingsAuth from '../components/settings/tabAuth'
import * as flags from './featureFlags';
class OAuthFeature extends SettingsFeatureBase {

  constructor(routes) {
    super();
    const route = {
      path: cfg.routes.settingsAuth,
      component: super.withMe(withDocTitle('Auth. Connectors', SettingsAuth))
    };

    routes.push(route);
  }

  componentDidMount() {
    this.init()
  }

  init(){
    if (!this.wasInitialized()) {
      this.startProcessing();
      fetchAuthProviders()
        .done(this.stopProcessing.bind(this))
        .fail(this.handleError.bind(this))
    }
  }

  isEnabled(){
    return flags.isAuthConnectorsEnabled();
  }

  onload() {
    const navItem = {
      to: cfg.routes.settingsAuth,
      title: "Auth. Connectors"
    }
    if (this.isEnabled()) {
      addNavItem(navItem);
      this.init();
    }
  }
}

export default OAuthFeature;