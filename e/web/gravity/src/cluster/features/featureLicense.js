import { addTopNavItem } from 'gravity/cluster/flux/nav/actions';
import withFeature, { FeatureBase } from 'gravity/components/withFeature';
import * as Icons from 'design/Icon';
import cfg from 'e-gravity/config'
import License from './../components/License';

class LicenseFeature extends FeatureBase {
  constructor() {
    super()
    this.Component = withFeature(this)(License);
  }

  getRoute(){
    return {
      title: 'License',
      path: cfg.routes.siteLicense,
      exact: true,
      component: this.Component
    }
  }

  onload({featureFlags}) {
    if(!featureFlags.clusterLicense()){
      this.setDisabled();
      return;
    }

    addTopNavItem({
      title: 'License',
      Icon: Icons.License,
      to: cfg.getSiteLicenseRoute()
    });
  }
}

export default LicenseFeature