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

import { map } from 'lodash';
import api from 'gravity/services/api';
import cfg from 'gravity/config';
import makeOperation from './makeOperation';
import makeProgress from './makeProgress';

const service = {

  fetchProgress({siteId, opId}){
    const url = cfg.getOperationProgressUrl(siteId, opId);
    return api.get(url).then(json => {
      return makeProgress(json)
    })
  },

  fetchOps(siteId, opId) {
    const url = cfg.getOperationUrl({siteId, opId});
    return api.get(url).then(json => map(json, makeOperation))
  },

  shrink(siteId, hostname) {
    const request = {
      servers: [hostname],
    };

    return api.post(cfg.getShrinkSiteUrl(siteId), request);
  }

}

export default service;