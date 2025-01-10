/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import cfg from 'teleport/config';
import api from 'teleport/services/api';
import session from 'teleport/services/websession';

import { MfaChallengeResponse } from '../mfa';
import { makeResetToken } from './makeResetToken';
import makeUser, { makeUsers } from './makeUser';
import makeUserContext from './makeUserContext';
import {
  ExcludeUserField,
  ResetPasswordType,
  User,
  UserContext,
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
  createUser(
    user: User,
    excludeUserField: ExcludeUserField,
    mfaResponse?: MfaChallengeResponse
  ) {
    return api
      .post(
        cfg.getUsersUrl(),
        withExcludedField(user, excludeUserField),
        null,
        mfaResponse
      )
      .then(makeUser);
  },

  createResetPasswordToken(
    name: string,
    type: ResetPasswordType,
    mfaResponse?: MfaChallengeResponse
  ) {
    return api
      .post(cfg.api.resetPasswordTokenPath, { name, type }, null, mfaResponse)
      .then(makeResetToken);
  },

  deleteUser(name: string) {
    return api.delete(cfg.getUserWithUsernameUrl(name));
  },

  async reloadUser(signal?: AbortSignal) {
    await session.renewSession({ reloadUser: true }, signal);
  },

  async checkUserHasAccessToAnyRegisteredResource() {
    const clusterId = cfg.proxyCluster;

    const res = await api
      .get(
        cfg.getUnifiedResourcesUrl(clusterId, {
          limit: 1,
          sort: {
            fieldName: 'name',
            dir: 'ASC',
          },
          includedResourceMode: 'all',
        })
      )
      .catch(err => {
        // eslint-disable-next-line no-console
        console.error('Error checking access to registered resources', err);
        return { items: [] };
      });

    return !!res?.items?.some?.(Boolean);
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
