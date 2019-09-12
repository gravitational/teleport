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

import $ from 'jquery';
import api from 'gravity/services/api';
import cfg from 'gravity/config';
import { RemoteAccessEnum } from 'gravity/services/enums';
import makeInfo from './makeInfo';
import makeRemoteStatus from './makeRemoteStatus';
import makeJoinToken from './makeJoinToken';

const service = {

  fetchInfo() {
    return api.get(cfg.getSiteInfoUrl()).then(makeInfo);
  },

  fetchRemoteAccess() {
    if(!cfg.isEnterprise){
      // return NA for open source version of the product
      return $.Deferred().resolve(makeRemoteStatus( { status: RemoteAccessEnum.NA }));
    }

    return api.get(cfg.getSiteRemoteAccessUrl()).then(makeRemoteStatus);
  },

  changeRemoteAccess(enabled) {
    const request = {
      enabled: enabled === true
    }

    return api.put(cfg.getSiteRemoteAccessUrl(), request)
      .then(makeRemoteStatus);
  },

  fetchJoinToken(){
    return api.get(cfg.getSiteTokenJoinUrl()).then(makeJoinToken);
  }

}

export default service;


