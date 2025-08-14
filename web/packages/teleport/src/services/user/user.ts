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

import cfg, { UrlListUsersParams } from 'teleport/config';
import api from 'teleport/services/api';
import session from 'teleport/services/websession';

import { MfaChallengeResponse } from '../mfa';
import { isPathNotFoundError } from '../version/unsupported';
import { makeResetToken } from './makeResetToken';
import makeUser, { makeUsers } from './makeUser';
import makeUserContext from './makeUserContext';
import {
  CreateUserVariables,
  ExcludeUserField,
  ResetPasswordType,
  User,
  UserContext,
  type UpdateUserVariables,
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

  // TODO(rudream): DELETE IN v21.0
  fetchUsers(signal?: AbortSignal) {
    return api.get(cfg.getUsersUrl(), signal).then(makeUsers);
  },

  async fetchUsersV2(
    params?: UrlListUsersParams,
    signal?: AbortSignal
  ): Promise<{
    items: User[];
    startKey: string;
  }> {
    return await api
      .get(cfg.getUsersUrlV2(params), signal)
      .then(res => {
        return {
          items: makeUsers(res.items),
          startKey: res.startKey,
        };
      })
      .catch(err => {
        // If this v2 paginated endpoint isn't found, fallback to the v1 endpoint but paginate locally in order to
        // maintain compatibility with the paginated table component which expects a paginated response.
        // TODO(rudream): DELETE IN v21.0
        if (isPathNotFoundError(err)) {
          return this.fetchUsers().then(users =>
            makeUsersPageLocally(params, users)
          );
        } else {
          throw err;
        }
      });
  },

  /**
   * Update user.
   * use allTraits to create new or replace entire user traits.
   * use traits to selectively add/update user traits.
   * @param user
   * @returns user
   */
  updateUser({ user, excludeUserField }: UpdateUserVariables) {
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
  createUser({ user, excludeUserField, mfaResponse }: CreateUserVariables) {
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

/**
 * makeUsersPageLocally mocks a paginated response for users so that a list of all users
 * can be handled by a serverside paginated table component.
 */
// TODO(rudream): DELETE IN v21.0
function makeUsersPageLocally(
  params: UrlListUsersParams,
  allUsers: User[]
): {
  items: User[];
  startKey: string;
} {
  if (params.search) {
    allUsers = allUsers.filter(u =>
      u.name.toLowerCase().includes(params.search.toLowerCase())
    );
  }

  if (params.startKey) {
    const startIndex = allUsers.findIndex(p => p.name === params.startKey);
    allUsers = allUsers.slice(startIndex);
  }

  const limit = params.limit || 200;
  const nextKey = allUsers.at(limit)?.name;
  allUsers = allUsers.slice(0, limit);

  return { items: allUsers, startKey: nextKey };
}

export default service;
