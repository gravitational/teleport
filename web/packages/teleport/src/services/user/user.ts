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
import {
  User,
  UserContext,
  ResetPasswordType,
  ExcludeUserField,
} from './types';

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

  fetchAccessGraphFeatures(): Promise<object> {
    return api.get(cfg.getAccessGraphFeaturesUrl());
  },

  fetchUser(username: string) {
    return api.get(cfg.getUserWithUsernameUrl(username)).then(makeUser);
  },

  fetchUsers() {
    return api.get(cfg.getUsersUrl()).then(makeUsers);
  },

  /**
   * Update user.
   * use allTraits to create new or replace entire user traits.
   * use traits to selectively add/update user traits.
   * @param user
   * @returns user
   */
  updateUser(user: User, excludeUserField: ExcludeUserField) {
    return api
      .put(cfg.getUsersUrl(), withExcludedField(user, excludeUserField))
      .then(makeUser);
  },

  /**
   * Create user.
   * use allTraits to create new or replace entire user traits.
   * use traits to selectively add/update user traits.
   * @param user
   * @returns user
   */
  createUser(user: User, excludeUserField: ExcludeUserField) {
    return api
      .post(cfg.getUsersUrl(), withExcludedField(user, excludeUserField))
      .then(makeUser);
  },

  createResetPasswordToken(name: string, type: ResetPasswordType) {
    return api
      .post(cfg.api.resetPasswordTokenPath, { name, type })
      .then(makeResetToken);
  },

  deleteUser(name: string) {
    return api.delete(cfg.getUserWithUsernameUrl(name));
  },

  async reloadUser(signal?: AbortSignal) {
    await session.renewSession({ reloadUser: true }, signal);
  },

  checkUserHasAccessToRegisteredResource(): Promise<boolean> {
    return api
      .get(cfg.getCheckAccessToRegisteredResourceUrl())
      .then(res => Boolean(res.hasResource));
  },

  fetchConnectMyComputerLogins(signal?: AbortSignal): Promise<Array<string>> {
    return api
      .get(cfg.getConnectMyComputerLoginsUrl(), signal)
      .then(res => res.logins);
  },
};

function withExcludedField(user: User, excludeUserField: ExcludeUserField) {
  const userReq = { ...user };
  switch (excludeUserField) {
    case ExcludeUserField.AllTraits: {
      delete userReq.allTraits;
      break;
    }
    case ExcludeUserField.Traits: {
      delete userReq.traits;
      break;
    }
    default: {
      excludeUserField satisfies never;
    }
  }

  return userReq;
}

export default service;
