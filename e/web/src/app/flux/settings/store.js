import { Store } from 'nuclear-js';
import { Record } from 'immutable';
import { SETTINGS_INIT } from './actionTypes';

class SettingsRec extends Record({  
  isInitialized: false
}){
  constructor(params){
    super(params);            
  }
  
  isReady() {
    return this.isInitialized;
  }    
}

export default Store({
  getInitialState() {
    return new SettingsRec();
  },

  initialize() {
    this.on(SETTINGS_INIT, state => {      
      return state.set('isInitialized', true);
    })
  }
});
