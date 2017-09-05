import FeatureBase from 'telebase-app/featureBase';
import cfg from 'app/config'
import { addNavItem } from './../flux/settings/actions';
import { fetchAuthProviders } from './../flux/settingsAuth/actions';
import SettingsAuth from '../components/settings/tabAuth'
import * as flags from './featureFlags';

class OAuthFeature extends FeatureBase {

  constructor(routes) {        
    super();
    const route = {
      title: 'Auth. Connectors',  
      path: cfg.routes.settingsAuth,
      component: super.withMe(SettingsAuth)
    };

    routes.push(route);        
  }
  
  getIndexRoute(){
    return cfg.routes.settingsAuth;
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

  onload() {             
    const navItem = {      
      to: cfg.routes.settingsAuth,
      title: "Auth. Connectors"  
    }        
    if (flags.isAuthConnectorsEnabled()) {
      addNavItem(navItem);
      this.init();
    }                
  }  
}

export default OAuthFeature;