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

import * as featureFlags from './flags';
import { SettingsFeatureBase }  from './featureSettings';
import cfg from 'app/config'
import { addNavItem } from './../../flux/settings/actions';
import SettingsAccount from './../../components/settings/accountTab'

const featureUrl = cfg.routes.settingsAccount;

class AccountFeature extends SettingsFeatureBase {

  constructor(routes) {        
    super();
    const route = {
      title: 'Account',  
      path: featureUrl,
      component: this.withMe(SettingsAccount)
    };

    routes.push(route);        
  }
      
  isEnabled() {
    return featureFlags.isAccountEnabled()
  }

  init(){
    if (!this.wasInitialized()) {
      this.stopProcessing();
    }      
  }

  onload() {                 
    if (!this.isEnabled()) {
      return;
    }

    const navItem = {      
      to: featureUrl,
      title: "Account"  
    }        
        
    addNavItem(navItem);
    this.init();                                  
  }  
}

export default AccountFeature;

