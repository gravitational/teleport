import withFeature, { FeatureBase } from 'gravity/components/withFeature';
import cfg from 'e-gravity/config'
import HubClusters from './../components/HubClusters';
import { addTopNavItem } from './../flux/nav/actions';
import { fetchClusters } from './../flux/actions';

class FeatureHubClusters extends FeatureBase {

  constructor(){
    super();
    this.Component = withFeature(this)(HubClusters);
  }

  getRoute(){
    return {
      title: 'Clusters',
      path: cfg.routes.hubClusters,
      exact: true,
      component: this.Component
    }
  }

  onload({ featureFlags }) {
    const allowed = featureFlags.hubClusters();
    if(!allowed){
      this.setDisabled();
      return;
    }

    addTopNavItem({
      title: 'Clusters',
      to: cfg.routes.hubClusters,
      exact: true
    });

    this.setProcessing();
    fetchClusters()
      .done(this.setReady.bind(this))
      .fail(this.setFailed.bind(this));
  }
}

export default FeatureHubClusters;