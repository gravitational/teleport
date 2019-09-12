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
import cfg from 'gravity/config';
import { getAccessToken } from 'gravity/services/api';
import { TLPT_TERMINAL_INIT, TLPT_TERMINAL_UPDATE_SESSION, TLPT_TERMINAL_SET_STATUS } from './actionTypes';

const TermStatusRec = new Record({
  isReady: false,
  isLoading: false,
  isError: false,
  errorText: undefined,
})
export class TermRec extends Record({
  status: TermStatusRec(),
  isNew: false,
  namespace: null,
  pod: null,
  container: null,
  hostname: null,
  login: null,
  siteId: null,
  serverId: null,
  sid: null,
  parties: []
}) {

  updateSession(json){
    return this.merge({
      ...json
    })
  }

  getClusterName() {
    return this.siteId;
  }

  getTtyConfig(){
    let url = '';

    const ttyParams = {
      login: this.login,
      sid: this.sid
    };

    if(this.pod){
      url = cfg.api.ttyWsK8sPodAddr,
      ttyParams.pod = {
        name: this.pod,
        container: this.container,
        namespace: this.namespace,
      }
    }else{
      url = cfg.api.ttyWsAddr,
      ttyParams.server_id = this.serverId;
    }

    const ttyUrl = url
      .replace(':fqdm', getHostName())
      .replace(':token', getAccessToken())
      .replace(':cluster', this.siteId)

    return {
      ttyUrl,
      ttyParams
    };
  }

  setStatus(json) {
    return this.setIn(['status'], new TermStatusRec(json));
  }

  getServerLabel() {
    if(this.pod){
      return `${this.login}@${this.pod}`;
    }

    if (this.hostname) {
      return `${this.login}@${this.hostname}`;
    }

    if (this.serverId) {
      return `${this.login}@${this.serverId}`;
    }

    return 'Connecting...';
  }

  init(json){
    return this.merge({
      ...json,
    })
  }

}

export default Store({
  getInitialState() {
    return new TermRec();
  },

  initialize() {
    this.on(TLPT_TERMINAL_SET_STATUS, (state, json) => state.setStatus(json));
    this.on(TLPT_TERMINAL_UPDATE_SESSION, (state, json) => state.updateSession(json));
    this.on(TLPT_TERMINAL_INIT, (state, json) => state.init(json));
  }
});

function getHostName(){
  return location.hostname+(location.port ? ':'+location.port: '');
}