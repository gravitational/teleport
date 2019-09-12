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
import makeCluster from './makeCluster';

export const service = {

  delete({siteId, secretKey, accessKey, sessionToken}){
    const request = {
      force: true,
      variables: {
        'secret_key': secretKey,
        'access_key': accessKey,
        'session_token': sessionToken
      }
    }

    return api.delete(cfg.getSiteUrl({siteId}), request);
  },

  unlink(siteId){
    const request = {
      remove_only: true
    }

    return api.delete(cfg.getSiteUrl({siteId}), request);
  },

  fetchCluster({ siteId, shallow = true } = {}){
    siteId = siteId || cfg.defaultSiteId;
    return api.get(cfg.getSiteUrl({siteId, shallow})).then(makeCluster);
  },

  fetchClusters({ shallow = true } = {}){
    return api.get(cfg.getSiteUrl({shallow}))
      .then(json => {
        const array = Array.isArray(json) ? json : [json];
        return map(array, makeCluster);
      })
  },

  updateLicense(license){
    const req = {
      license
    }

    return api.put(cfg.getSiteLicenseUrl(), req);
  }
}