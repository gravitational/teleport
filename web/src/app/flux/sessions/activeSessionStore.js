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
import { Record, List } from 'immutable';

import {
  RECEIVE_ACTIVE_SESSIONS,
  UPDATE_ACTIVE_SESSION } from './actionTypes';

const ActiveSessionRec = Record({ 
  id: undefined,
  namespace: undefined,  
  login: undefined,
  active: undefined,
  created: undefined,
  last_active: undefined,
  server_id: undefined,
  siteId: undefined,
  parties: List()  
})

const PartyRecord = Record({
  user: undefined,
  serverIp: undefined,
  serverId: undefined
})

const defaultState = () => toImmutable({}); 

export default Store({
  getInitialState() {
    return defaultState();
  },

  initialize() {    
    this.on(RECEIVE_ACTIVE_SESSIONS, receive);
    this.on(UPDATE_ACTIVE_SESSION, updateSession);
  }
})

function updateSession(state, { siteId, json }) {
  let rec = createSessionRec(siteId, json);  
  return state.set(rec.id, rec);
}

function receive(state, { siteId, json }) {
  let jsonArray = json || [];
  let newState = defaultState();  
  jsonArray
    .filter(item => item.active === true)
    .forEach(item => {
      let rec = createSessionRec(siteId, item);      
      newState = newState.set(rec.id, rec);  
    });
  
  return newState;  
}

function createSessionRec(siteId, json) {
  let parties = createParties(json.parties);
  let rec = new ActiveSessionRec(toImmutable({
      ...json,
    siteId,
    parties
  }));

  return rec;
}

function createParties(jsonArray) {
  jsonArray = jsonArray || [];
  return jsonArray.map(item =>    
    new PartyRecord({
      user: item.user,
      serverIp: item.remote_addr,
      serverId: item.server_id
    })
  );   
}
