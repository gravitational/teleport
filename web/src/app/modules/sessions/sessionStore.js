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
  TLPT_SESSINS_RECEIVE,
  TLPT_SESSINS_UPDATE,
  TLPT_SESSINS_REMOVE_STORED,
  TLPT_SESSINS_UPDATE_WITH_EVENTS }  = require('./actionTypes');

export default Store({
  getInitialState() {
    return toImmutable({});
  },

  initialize() {
    this.on(TLPT_SESSINS_UPDATE_WITH_EVENTS, updateSessionWithEvents);
    this.on(TLPT_SESSINS_RECEIVE, receiveSessions);
    this.on(TLPT_SESSINS_UPDATE, updateSession);
    this.on(TLPT_SESSINS_REMOVE_STORED, removeStoredSessions);
  }
})

function removeStoredSessions(state){
  return state.withMutations(state => {
    state.valueSeq().forEach(item=> {
      if(item.get('active') !== true){
        state.remove(item.get('id'));
      }
    });
  });
}

function updateSessionWithEvents(state, events){
  return state.withMutations(state => {
    events.forEach(item=>{
      if(item.event !== 'session.start' && item.event !== 'session.end'){
        return;
      }

      if(state.getIn([item.sid, 'active']) === true){
        return;
      }

      // check if record already exists
      let session = state.get(item.sid);
      if(!session){
         session = { id: item.sid };
      }else{
        session = session.toJS();
      }

      session.login = item.user;

      if(item.event === 'session.start'){
        session.created = item.time;
      }

      if(item.event === 'session.end'){
        session.last_active = item.time;
      }

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
    jsonArray.forEach((item) => {
      item.isActive = true;
      item.created = new Date(item.created);
      item.last_active = new Date(item.last_active);
      state.set(item.id, toImmutable(item))
    })
  });
}
