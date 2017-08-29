import cfg from 'app/config'
import FeatureBase from './../featureBase';
import { addNavItem } from './../flux/settings/actions';
import { fetchAuthProviders } from './../flux/settingsAuth/actions';
import SettingsAuth from '../components/settings/tabAuth'

class OAuthFeature extends FeatureBase {

  constructor(routes) {        
    super();
    const route = {
      path: cfg.routes.settingsAuth,
      component: super.withMe(SettingsAuth)
    };

    routes.push(route);        
  }
  
  getIndexRoute(){
    return cfg.routes.settingsAuth;
  }

  onload() {         
    const navItem = {      
      to: cfg.routes.settingsAuth,
      title: "Auth. Connectors"  
    }
    
    addNavItem(navItem);

    this.startProcessing();
    
    fetchAuthProviders().always(this.stopProcessing.bind(this))      
  }  
}

export default OAuthFeature;