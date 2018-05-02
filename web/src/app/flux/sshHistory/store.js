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

import { Store } from 'nuclear-js';
import { Record, Map } from 'immutable';
import { ADD_ITEM }from './actionTypes';

const STORE_NAME = 'tlpt_ssh_history';

export class SshHistoryRec extends Record({    
  clusterLogins: new Map() 
}){
  constructor(params) {    
    super(params);            
  }
   
  getPrevLogins(siteId) {
    return this.clusterLogins.get(siteId) || [];    
  }

  addLoginString({ login, serverId, siteId }) {    
    let logins = this.getIn(['clusterLogins', siteId]);
    if (!logins) {
      logins = [];
    }

    const loginStr = `${login}@${serverId}`;
    const exists = logins.some(i => i === loginStr);

    if (exists) {
      return this;
    }

    logins.unshift(loginStr);
    return this.setIn(['clusterLogins', siteId], logins);
  }
}

const store = Store({
  getInitialState() {
    return new SshHistoryRec();
  },

  initialize() {        
    this.on(ADD_ITEM, (state, params) => state.addLoginString(params));
  }
});

export const register = reactor => {
  reactor.registerStores({
    [STORE_NAME]: store
  })  
}

export const getters = {
  store: [STORE_NAME]
}