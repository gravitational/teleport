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

import { Store, toImmutable } from 'nuclear-js';
import { Record, Map, List } from 'immutable';
import { RECEIVE_USERACL } from './actionTypes';

const sortLogins = loginList => {
  let index = loginList.indexOf('root');
  if (index !== -1) {
    loginList = loginList.remove(index);
    return loginList.sort().unshift('root')
  }

  return loginList;
}

class AccessRec extends Record({  
  admin: Map({
    enabled: false
  }),
  ssh: Map({
    enabled: false,
    logins: List()
  })
}){
  constructor(params) {    
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

    return logins.toJS()    
  }
}

export default Store({
  getInitialState() {
    return new AccessRec();
  },

  initialize() {          
    this.on(RECEIVE_USERACL, receiveAcl);            
  }
})

function receiveAcl(state, json) {
  json = json || {};   
  let aclMap = toImmutable(json);
  let loginList = aclMap.getIn(['ssh', 'logins']);
  if (loginList) {
    aclMap = aclMap.setIn(['ssh', 'logins'], sortLogins(loginList));
  }

  return new AccessRec(aclMap);    
}
