import FeatureBase from 'telebase-app/featureBase';
import cfg from 'app/config'
import { addNavItem } from './../flux/settings/actions';
import { fetchRoles } from './../flux/settingsRoles/actions';
import SettingsRoles from '../components/settings/tabRoles'
import * as flags from './featureFlags';

class RolesFeature extends FeatureBase {

  constructor(routes) {        
    super();
    const route = {
      title: 'Roles',  
      path: cfg.routes.settingsRoles,
      component: super.withMe(SettingsRoles)
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
    fetchRoles()
      .done(this.stopProcessing.bind(this))
      .fail(this.handleError.bind(this))                         
  }
  
  getIndexRoute(){
    return cfg.routes.settingsRoles;
  }

  onload() {         
    if (!flags.isRolesEnabled()) {
      return false;
    }
    
    const navItem = {      
      to: cfg.routes.settingsRoles,
      title: "Roles"  
    }
            
    addNavItem(navItem);    
  }  
}

export default RolesFeature;