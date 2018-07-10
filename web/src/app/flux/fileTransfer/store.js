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
import * as AT from './actionTypes';

class FileTransferStore extends Record({
  isOpen: false,
  isUpload: false,
  siteId: '',
  serverId: '',
  login: '',
}){

  constructor(params){
    super(params);
  }

  close() {
    return this.set('isOpen', false);
  }

  open({isUpload, siteId, serverId, login}) {
    return this.merge({
      isOpen: true,
      isUpload,
      siteId,
      serverId,
      login
    })
  }
}

export default Store({
  getInitialState() {
    return new FileTransferStore();
  },

  initialize() {
    this.on(AT.OPEN, (state, json) => state.open(json));
    this.on(AT.CLOSE, state => state.close());
  }
})