// telebase imports
import FeatureBase from 'telebase-app/featureBase';
import FeatureActivator from 'telebase-app/featureActivator';
import { addNavItem } from 'telebase-app/flux/app/actions';
import cfg from 'app/config'
import OAuthFeature from './settingsAuth';
import TrustedClustersFeature from './settingsClusters';
import RolesFeature from './settingsRoles';
import Settings from '../components/settings/main'
import { initSettings } from './../flux/settings/actions'
import SettingsIndex from '../components/settings'
import * as API from 'app/flux/restApi/constants';
import * as flags from './featureFlags';

const nestedFeatureActivator = new FeatureActivator();
const nestedRoutes = [];

nestedFeatureActivator.register(new OAuthFeature(nestedRoutes));
nestedFeatureActivator.register(new RolesFeature(nestedRoutes));
nestedFeatureActivator.register(new TrustedClustersFeature(nestedRoutes));

const settingsNavItem = {
  icon: 'fa fa-wrench',
  to: cfg.routes.settingsBase,
  title: 'Settings'
}

export default class SettingsFeature extends FeatureBase {
  constructor(routes) {        
    super(API.TRYING_TO_INIT_SETTINGS);    
    const settingsRoutes =  {
      path: cfg.routes.settingsBase,
      title: 'Settings',  
      component: super.withMe(Settings),
      indexRoute: {     
        // need index component to handle default redirect to available nested feature
        component: SettingsIndex
      },  
      childRoutes: nestedRoutes
    }

    routes.push(settingsRoutes);        
  }

  componentDidMount() {                
    try{      
      initSettings(nestedFeatureActivator);               
    }catch(err){
      this.handleError(err);
    }    
  }

  onload() {                 
    if(flags.isAnythingEnabled()){
      addNavItem(settingsNavItem); 
    }    
  }  
}