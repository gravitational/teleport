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

import { makeCluster } from '../clusters';

import { makeAcl } from './makeAcl';
import { UserContext, AccessCapabilities, PasswordState } from './types';

export default function makeUserContext(json: any): UserContext {
  json = json || {};
  const username = json.userName;
  const authType = json.authType;
  const accessRequestId = json.accessRequestId;

  const cluster = makeCluster(json.cluster);
  const acl = makeAcl(json.userAcl);
  const accessStrategy = json.accessStrategy || defaultStrategy;
  const accessCapabilities = makeAccessCapabilities(json.accessCapabilities);
  const allowedSearchAsRoles = json.allowedSearchAsRoles || [];
  const passwordState =
    json.passwordState || PasswordState.PASSWORD_STATE_UNSPECIFIED;

  return {
    username,
    authType,
    acl,
    cluster,
    accessStrategy,
    accessCapabilities,
    accessRequestId,
    allowedSearchAsRoles,
    passwordState,
  };
}

function makeAccessCapabilities(json): AccessCapabilities {
  json = json || {};

  return {
    requestableRoles: json.requestableRoles || [],
    suggestedReviewers: json.suggestedReviewers || [],
  };
}

export const defaultStrategy = {
  type: 'optional',
  prompt: '',
};
