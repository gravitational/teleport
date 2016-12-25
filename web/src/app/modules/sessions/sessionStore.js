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

var { Store, toImmutable } = require('nuclear-js');
var {
  TLPT_SESSIONS_RECEIVE,
  TLPT_SESSIONS_UPDATE,
  TLPT_SESSIONS_UPDATE_WITH_EVENTS }  = require('./actionTypes');

var PORT_REGEX = /:\d+$/;

export default Store({
  getInitialState() {
    return toImmutable({});
  },

  initialize() {
    this.on(TLPT_SESSIONS_UPDATE_WITH_EVENTS, updateSessionWithEvents);
    this.on(TLPT_SESSIONS_RECEIVE, receiveSessions);
    this.on(TLPT_SESSIONS_UPDATE, updateSession);
  }
})

function getIp(addr){  
  addr = addr || '';
  return addr.replace(PORT_REGEX, '');
}

function updateSessionWithEvents(state, { jsonEvents=[], siteId }){
  return state.withMutations(state => {
    jsonEvents.forEach(item=>{
      if(item.event !== 'session.start' && item.event !== 'session.end'){
        return;
      }

      // check if record already exists
      let session = state.get(item.sid);
      if(!session){
         session = {};
      }else{
        session = session.toJS();
      }

      session.id = item.sid;
      session.user = item.user;

      if(item.event === 'session.start'){
        session.created = item.time;
        session.nodeIp = getIp(item['addr.local']);
        session.clientIp = getIp(item['addr.remote']);
      }

      if(item.event === 'session.end'){
        session.last_active = item.time;
        session.active = false;
        session.stored = true;
      }

      session.siteId = siteId;      
      state.set(session.id, toImmutable(session));
    })
  });
}

function updateSession(state, json){
  return state.set(json.id, toImmutable(json));
}

function receiveSessions(state, jsonArray){
  jsonArray = jsonArray || [];

  return state.withMutations(state => {
    jsonArray.forEach(item => {
      if(!state.getIn([item.id, 'stored'])){
        state.set(item.id, toImmutable(item))
      }
    })
  });
}
