import FeatureBase from 'telebase-app/featureBase';
import cfg from 'app/config'
import { addNavItem } from './../flux/settings/actions';
import { fetchTrustedClusters } from './../flux/settingsClusters/actions';
import TrustedClusters from '../components/settings/tabTrustedClusters'
import * as flags from './featureFlags';

class TrustedClustersFeature extends FeatureBase {

  constructor(routes) {        
    super();
    const route = {
      title: 'Trusted Clusters',  
      path: cfg.routes.settingsCluster,
      component: super.withMe(TrustedClusters)
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
    fetchTrustedClusters()
      .done(this.stopProcessing.bind(this))
      .fail(this.handleError.bind(this))                      
  }
  
  getIndexRoute(){
    return cfg.routes.settingsCluster;
  }

  onload() {             
    const navItem = {      
      to: cfg.routes.settingsCluster,
      title: "Trusted Clusters"  
    }
    
    if (flags.isTrustedClrsEnabled()) {
      addNavItem(navItem);
      this.init();
    }      
  }  
}

export default TrustedClustersFeature;