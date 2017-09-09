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
import reactor from 'app/reactor';
import { Store } from 'nuclear-js';
import { Record, List } from 'immutable';
import * as AT from './actionTypes';

class AppRec extends Record({  
  siteId: null,  
  navItems: new List() 
}){      

  constructor(props) {
    super(props)
  }

  setSiteId(siteId) {
    return this.set('siteId', siteId);
  }

  getClusterName() {
    return this.get('siteId');
  }

  getNavItems(){    
    return this.navItems.toJS();
  }
  
  addNavItem(navItem) {        
    return this.set('navItems', this.navItems.push(navItem))
  }
}

export function getStore() {
  return reactor.evaluate(['tlpt']);
}

export default Store({
  
  getInitialState() {
    return new AppRec();
  },

  initialize() {    
    this.on(AT.SET_SITE_ID, (state, siteId) => state.setSiteId(siteId) );            
    this.on(AT.ADD_NAV_ITEM, (state, navItem) => state.addNavItem(navItem))
  }
});

