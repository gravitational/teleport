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

import api from 'teleport/services/api';
import cfg from 'teleport/config';
import session from 'teleport/services/session';
import makeUser from './makeUser';
import makeAccessRequest from './makeAccessRequest';
import { User } from './types';

let cached: User = null;

const service = {
  createAccessRequest(reason?: string) {
    return api
      .post(cfg.getRequestAccessUrl(), { reason })
      .then(makeAccessRequest);
  },

  applyPermission(requestId?: string) {
    return session.renewSession(requestId);
  },

  fetchAccessRequest(requestId?: string) {
    return api.get(cfg.getRequestAccessUrl(requestId)).then(makeAccessRequest);
  },

  fetchUser(clusterId?: string, fromCache = true) {
    if (fromCache && cached) {
      return Promise.resolve(cached);
    }

    return api
      .get(cfg.getUserUrl(clusterId))
      .then(makeUser)
      .then(userContext => {
        cached = userContext;
        return cached;
      });
  },
};

export default service;
