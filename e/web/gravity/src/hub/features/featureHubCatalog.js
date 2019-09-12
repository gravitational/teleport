import withFeature, { FeatureBase } from 'gravity/components/withFeature';
import cfg from 'e-gravity/config'
import HubCatalog from './../components/HubCatalog';
import { addTopNavItem } from './../flux/nav/actions';
import { fetchApps } from './../flux/catalog/actions';

class FeatureHubCatalog extends FeatureBase {

  constructor(){
    super();
    this.Component = withFeature(this)(HubCatalog);
  }

  getRoute(){
    return {
      title: 'Catalog',
      path: cfg.routes.hubCatalog,
      exact: true,
      component: this.Component
    }
  }

  onload({ featureFlags }) {
    const allowed = featureFlags.hubApps();
    if(!allowed){
      this.setDisabled();
      return;
    }

    addTopNavItem({
      title: 'Catalog',
      to: cfg.routes.hubCatalog
    });

    this.setProcessing();
    fetchApps()
      .done(this.setReady.bind(this))
      .fail(this.setFailed.bind(this));
  }
}

export default FeatureHubCatalog;