import Certificate from 'gravity/cluster/components/Certificate'
import { fetchTlsCert } from 'gravity/cluster/flux/tlscert/actions';
import withFeature, { FeatureBase } from 'gravity/components/withFeature';
import * as featureFlags from 'gravity/cluster/featureFlags';
import cfg from 'e-gravity/config'
import { addSettingNavItem } from 'e-gravity/hub/flux/nav/actions';
import * as Icons from 'design/Icon';

class FeatureHubCertificate extends FeatureBase {

  constructor() {
    super();
    this.Component = withFeature(this)(Certificate);
  }

  getRoute(){
    return {
      title: 'Certificate',
      path: cfg.routes.hubSettingCert,
      exact: true,
      component: this.Component
    }
  }

  onload() {
    if(!featureFlags.clusterCert()){
      this.setDisabled();
      return;
    }

    addSettingNavItem({
      title: 'HTTPS Certificate',
      Icon: Icons.License,
      exact: true,
      to: cfg.routes.hubSettingCert
    });

    this.setProcessing();
    return fetchTlsCert(cfg.defaultSiteId)
      .done(this.setReady.bind(this))
      .fail(this.setFailed.bind(this));
  }
}

export default FeatureHubCertificate;