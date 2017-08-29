import cfg from 'app/config'
import FeatureBase from './../featureBase';
import { addNavItem } from './../flux/settings/actions';
import { fetchRoles } from './../flux/settingsRoles/actions';
import SettingsRoles from '../components/settings/tabRoles'

class RolesFeature extends FeatureBase {

  constructor(routes) {        
    super();
    const route = {
      path: cfg.routes.settingsRoles,
      component: super.withMe(SettingsRoles)
    };

    routes.push(route);        
  }
  
  getIndexRoute(){
    return cfg.routes.settingsRoles;
  }

  onload() {         
    const navItem = {      
      to: cfg.routes.settingsRoles,
      title: "Roles"  
    }
    
    addNavItem(navItem);

    this.startProcessing();
    
    fetchRoles()
      .done(this.stopProcessing.bind(this))
      .fail(this.handleError.bind(this))            
  }  
}

export default RolesFeature;