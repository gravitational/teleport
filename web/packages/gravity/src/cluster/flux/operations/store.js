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

import { Store } from 'nuclear-js';
import { Record } from 'immutable';
import { RECEIVE_OPERATIONS, RECEIVE_PROGRESS } from './actionTypes';
import { StatusEnum } from 'gravity/services/operations';

class OperationStoreRec extends Record({
  operations: [],
  progress: {},
}){
  getActive() {
    return this.operations.filter( o => o.status === StatusEnum.PROCESSING);
  }
}

export default Store({
  getInitialState() {
    return new OperationStoreRec();
  },

  initialize() {
    this.on(RECEIVE_OPERATIONS, receive);
    this.on(RECEIVE_PROGRESS, receiveProgress);
  }
})

function receive(state, sessions) {
  return state.set('operations', sessions);
}

function receiveProgress(state, progress) {
  const progressMap = {
    ...state.progress,
    [progress.opId]: progress
  }

  return state.set('progress', progressMap);
}