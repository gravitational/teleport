import { Store, toImmutable } from 'nuclear-js';
import { Record, Map, List } from 'immutable';
import { USERACL_RECEIVE } from './actionTypes';

class AccessRec extends Record({  
  admin: Map({
    enabled: false
  }),
  ssh: Map({
    enabled: false,
    logins: List()
  })
}){
  constructor(params){
    super(params);            
  }
  
  isAdminEnabled() {
    return this.getIn(['admin', 'enabled']);
  }
  
  isSshEnabled() {
    let logins = this.getIn(['ssh', 'logins']);
    return logins ? logins.size > 0 : false;    
  }

  getSshLogins() {
    let logins = this.getIn(['ssh', 'logins']);
    if (!logins) {
      return []
    }

    return logins.toJS();
  }
}

export default Store({
  getInitialState() {
    return new AccessRec();
  },

  initialize() {          
    this.on(USERACL_RECEIVE, receiveAcl);            
  }
})

function receiveAcl(state, json) {
  json = json || {};    
  return new AccessRec(toImmutable(json));    
}


