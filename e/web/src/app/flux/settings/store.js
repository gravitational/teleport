import { Store } from 'nuclear-js';
import { Record, List } from 'immutable';
import * as AT from './actionTypes';

class SettingsRec extends Record({  
  isInitialized: false,  
  navItems: new List() 
}){
  constructor(params){
    super(params);            
  }
  
  isReady() {
    return this.isInitialized;
  }    

  getNavItems(){    
    return this.navItems.toJS();
  }
  
  addNavItem(navItem) {    
    return this.set('navItems', this.navItems.push(navItem))
  }
}

export default Store({
  getInitialState() {
    return new SettingsRec();
  },

  initialize() {    
    this.on(AT.INIT, state => state.set('isInitialized', true))    
    this.on(AT.ADD_NAV_ITEM, (state, navItem) => state.addNavItem(navItem))
  }
});
