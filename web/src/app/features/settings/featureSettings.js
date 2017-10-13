/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import FeatureBase from './../../featureBase';
import FeatureActivator from './../../featureActivator';
import { addNavItem } from './../../flux/app/actions';
import cfg from 'app/config'

import Settings from './../../components/settings/main'
import { initSettings } from './../../flux/settings/actions'
import SettingsIndex from './../../components/settings'
import * as API from 'app/flux/restApi/constants';

const settingsNavItem = {
  icon: 'fa fa-wrench',
  to: cfg.routes.settingsBase,
  title: 'Settings'
}

/**
 * Describes nested features within Settings
 */
export class SettingsFeatureBase extends FeatureBase {
  constructor(props) {
    super(props)
  }

  isEnabled() {    
    return true;
  }
}

export default class SettingsFeature extends FeatureBase {
  
  featureActivator = new FeatureActivator();

  childRoutes = [];

  addChild(feature) {
    if (!(feature instanceof SettingsFeatureBase)) {
      throw Error('feature must implement SettingsFeatureBase');
    }

    this.featureActivator.register(feature)
  }

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
      childRoutes: this.childRoutes
    }

    routes.push(settingsRoutes);        
  }

  componentDidMount() {                
    try{      
      initSettings(this.featureActivator);               
    }catch(err){
      this.handleError(err);
    }    
  }

  onload() {                 
    const features = this.featureActivator.getFeatures();
    const some = features.some(f => f.isEnabled());
    if(some){
      addNavItem(settingsNavItem); 
    }    
  }  
}
