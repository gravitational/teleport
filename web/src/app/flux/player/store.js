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
import { Record } from 'immutable';
import cfg from 'app/config';

import {
  TLPT_PLAYER_INIT,
  TLPT_PLAYER_CLOSE,
  TLPT_PLAYER_SET_STATUS
} from './actionTypes';

const PlayerStatusRec = new Record({
  isReady: false,
  isLoading: false,
  isNotFound: false,
  isError: false,
  errorText: undefined,
})

export class PlayerRec extends Record({
  status: new PlayerStatusRec(),
  siteId: undefined,
  sid: undefined
}) {
  makeReady() {
    return this.set('status', new PlayerStatusRec({ isReady: true }));
  }

  isReady() {
    return this.getIn(['status', 'isReady']);
  }

  isLoading() {
    return this.getIn(['status', 'isLoading']);
  }

  isError() {
    return this.getIn(['status', 'isError']);
  }

  getErrorText() {
    return this.getIn(['status', 'errorText']);
  }
  
  getStoredSessionUrl() {
    return cfg.api.getFetchSessionUrl({
      siteId: this.siteId,
      sid: this.sid
    });    
  }
}
export default Store({
  getInitialState() {
    return new PlayerRec();
  },

  initialize() {
    this.on(TLPT_PLAYER_INIT, init);
    this.on(TLPT_PLAYER_CLOSE, close);
    this.on(TLPT_PLAYER_SET_STATUS, changeStatus);
  }
})

function close(){
  return new PlayerRec();
}

function init(state, json){
  return new PlayerRec(json).makeReady();
}

function changeStatus(state, status) {
  return state.setIn(['status'], new PlayerStatusRec(status));  
}
