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
import * as AT from './actionTypes';
import { Record } from 'immutable';

export const TrackRec = new Record({
  isProcessing: false,
  isFailed: false,
  isSuccess: false,
  message: ''
});

export default Store({

  getInitialState() {
    return toImmutable({});
  },

  initialize() {
    this.on(AT.START, start);
    this.on(AT.FAIL, fail);
    this.on(AT.SUCCESS, success);
    this.on(AT.CLEAR, clear);
  }
})

function start(state, request){
  return state.set(request.type, new TrackRec({isProcessing: true}));
}

function fail(state, request){
  return state.set(request.type, new TrackRec({isFailed: true, message: request.message}));
}

function success(state, request){
  return state.set(request.type, new TrackRec({isSuccess: true, message: request.message}));
}

function clear(state, request) {  
  return state.set(request.type, new TrackRec());
}
