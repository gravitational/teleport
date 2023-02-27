/*
Copyright 2019-2020 Gravitational, Inc.

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
import session from 'teleport/services/websession';

import makeUserContext from './makeUserContext';
import { makeResetToken } from './makeResetToken';
import makeUser, { makeUsers } from './makeUser';
import { User, UserContext, ResetPasswordType } from './types';

const cache = {
  userContext: null as UserContext,
};

const service = {
  fetchUserContext(fromCache = true) {
    if (fromCache && cache['userContext']) {
      return Promise.resolve(cache['userContext']);
    }

    return api
      .get(cfg.getUserContextUrl())
      .then(makeUserContext)
      .then(userContext => {
        cache['userContext'] = userContext;
        return cache['userContext'];
      });
  },

  fetchUser(username: string) {
    return api.get(cfg.getUserWithUsernameUrl(username)).then(makeUser);
  },

  fetchUsers() {
    return api.get(cfg.getUsersUrl()).then(makeUsers);
  },

  updateUser(user: User) {
    return api.put(cfg.getUsersUrl(), user).then(makeUser);
  },

  createUser(user: User) {
    return api.post(cfg.getUsersUrl(), user).then(makeUser);
  },

  createResetPasswordToken(name: string, type: ResetPasswordType) {
    return api
      .post(cfg.api.resetPasswordTokenPath, { name, type })
      .then(makeResetToken);
  },

  deleteUser(name: string) {
    return api.delete(cfg.getUserWithUsernameUrl(name));
  },

  applyUserTraits() {
    return session.renewSession({ reloadUser: true });
  },

  checkUserHasAccessToRegisteredResource(): Promise<boolean> {
    return api
      .get(cfg.getCheckAccessToRegisteredResourceUrl())
      .then(res => Boolean(res.hasResource));
  },
};

export default service;
