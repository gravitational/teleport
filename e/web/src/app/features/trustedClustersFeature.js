import cfg from 'app/config'
import FeatureBase from './../featureBase';
import { addNavItem } from './../flux/settings/actions';
import { fetchTrustedClusters } from './../flux/settingsClusters/actions';
import TrustedClusters from '../components/settings/tabTrustedClusters'

class TrustedClustersFeature extends FeatureBase {

  constructor(routes) {        
    super();
    const route = {
      path: cfg.routes.settingsCluster,
      component: super.withMe(TrustedClusters)
    };

    routes.push(route);        
  }
  
  getIndexRoute(){
    return cfg.routes.settingsCluster;
  }

  onload() {         
    const navItem = {      
      to: cfg.routes.settingsCluster,
      title: "Trusted Clusters"  
    }
    
    addNavItem(navItem);

    this.startProcessing();

    fetchTrustedClusters()
      .done(this.stopProcessing.bind(this))
      .fail(this.handleError.bind(this))            
  }  
}

export default TrustedClustersFeature;