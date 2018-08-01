import { SettingsFeatureBase } from 'telebase-app/features/settings/featureSettings';
import { addNavItem } from 'telebase-app/flux/settings/actions';
import { withDocTitle } from 'telebase-app/components/documentTitle';

import cfg from './../config'
import { fetchTrustedClusters } from './../flux/settingsClusters/actions';
import TrustedClusters from '../components/settings/tabTrustedClusters'
import * as flags from './featureFlags';

class TrustedClustersFeature extends SettingsFeatureBase {

  constructor(routes) {
    super();
    const route = {
      path: cfg.routes.settingsCluster,
      component: super.withMe(withDocTitle('Trusted Clusters', TrustedClusters))
    };

    routes.push(route);
  }

  componentDidMount() {
    this.init()
  }

  isEnabled(){
    return flags.isTrustedClrsEnabled();
  }

  init(){
    if (this.wasInitialized()) {
      return;
    }

    this.startProcessing();
    fetchTrustedClusters()
      .done(this.stopProcessing.bind(this))
      .fail(this.handleError.bind(this))
  }

  onload() {
    const navItem = {
      to: cfg.routes.settingsCluster,
      title: "Trusted Clusters"
    }

    if (this.isEnabled()) {
      addNavItem(navItem);
      this.init();
    }
  }
}

export default TrustedClustersFeature;