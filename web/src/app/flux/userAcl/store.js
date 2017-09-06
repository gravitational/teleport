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
import { Store, toImmutable } from 'nuclear-js';
import { Record, List } from 'immutable';
import { RECEIVE_USERACL } from './actionTypes';

// sort logins by making 'root' as the first in the list
const sortLogins = loginList => {
  let index = loginList.indexOf('root');
  if (index !== -1) {
    loginList = loginList.remove(index);
    return loginList.sort().unshift('root')
  }

  return loginList;
}

const Access = new Record({
  list: false,
  read: false,
	edit: false,
	create: false,
	remove: false
})
	
class AccessListRec extends Record({  
  authConnectors: new Access(),
  trustedClusters: new Access(),
  roles: new Access(),
  sessions: new Access(),
  sshLogins: new List()
}){
  constructor(json = {}) {    
    const map = toImmutable(json);    
    const sshLogins = new List(map.get('sshLogins'));            
    const params = {
      sshLogins: sortLogins(sshLogins),
      authConnectors: new Access(map.get('authConnectors')),
      trustedClusters: new Access(map.get('trustedClusters')),
      roles: new Access(map.get('roles')),
      sessions: new Access(map.get('sessions'))
    }
      
    super(params);                
  }
        
  getSessionAccess() {
    return this.get('sessions');
  }

  getRoleAccess() {
    return this.get('roles');    
  }

  getConnectorAccess() {
    return this.get('authConnectors');    
  }
  
  getClusterAccess() {
    return this.get('trustedClusters');    
  }
      
  getSshLogins() {
    return this.get('sshLogins')    
  }
}

export function getAcl() {
  return reactor.evaluate(['tlpt_user_acl']);
}

export default Store({
  getInitialState() {
    return new AccessListRec();
  },

  initialize() {          
    this.on(RECEIVE_USERACL, (state, json ) => new AccessListRec(json) );            
  }
})