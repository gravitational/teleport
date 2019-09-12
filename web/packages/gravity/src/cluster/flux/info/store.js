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

import { RemoteAccessEnum } from 'gravity/services/enums';
import { Store } from 'nuclear-js';
import { Record } from 'immutable';
import * as actionTypes from './actionTypes';

const StoreRec = Record({
  remoteAccess: RemoteAccessEnum.NA,
  info: {},
});

export default Store({
  getInitialState() {
    return new StoreRec();
  },

  initialize() {
    this.on(actionTypes.SITE_SET_REMOTE_STATUS, setRemoteStatus);
    this.on(actionTypes.SITE_RECEIVE_INFO, receiveInfo);
  }
})

function receiveInfo(state, info) {
  return state.set('info', info);
}

function setRemoteStatus(state, { status }) {
  return state.set('remoteAccess', status);
}