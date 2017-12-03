import { Store } from 'nuclear-js';
import * as AT from './actionTypes';
import { Record } from 'immutable';

const SeverityEnum = {
  INFO: 'info',
  ERROR: 'error',
  WARNING: 'warning'
}

class LicenseStatusRec extends Record({
  html: '',
  text: '',
  type: '',
  severity: ''
}){

  isWarning(){
    return this.severity === SeverityEnum.WARNING;
  }

  isError(){
    return this.severity === SeverityEnum.ERROR;
  }

  isInfo(){
    return this.severity === SeverityEnum.INFO;
  }
}
  
export default Store({
  getInitialState() {
    return null;
  },

  initialize() {        
    this.on(AT.RECEIVE_STATUS, (state, json) => new LicenseStatusRec(json) );                
  }
})

