/*
Copyright 2019 Gravitational, Inc.

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

import { keyBy, values } from 'lodash';
import { Store } from 'nuclear-js';
import { Record } from 'immutable';
import { CLUSTER_EVENTS_RECEIVE, CLUSTER_EVENTS_RECEIVE_LATEST } from './actionTypes';

export class StoreRec extends Record({
  overflow: false,
  events: {}
}){

  mergeEvents(events){
    events = {
      ...this.events,
      ...keyBy(events, 'id')
    }

    return this.set('events', events)
  }

  getEvents(){
    return values(this.events);
  }
}

export default Store({
  getInitialState() {
    return new StoreRec();
  },

  initialize() {
    this.on(CLUSTER_EVENTS_RECEIVE_LATEST, (state, events) => state.mergeEvents(events));
    this.on(CLUSTER_EVENTS_RECEIVE, receiveEvents);
  }
})

function receiveEvents(state, { overflow, events }) {
  state = state.mergeEvents(events)
  return state.set('overflow', overflow);
}