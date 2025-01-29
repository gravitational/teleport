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

import cfg, { UrlResourcesParams } from 'teleport/config';
import { ResourcesResponse } from 'teleport/services/agents';
import api from 'teleport/services/api';

import { makeUserGroup } from './makeUserGroup';
import type { UserGroup } from './types';

class UserGroupService {
  fetchUserGroups(
    clusterId: string,
    params: UrlResourcesParams,
    signal?: AbortSignal
  ): Promise<ResourcesResponse<UserGroup>> {
    return api
      .get(cfg.getUserGroupsListUrl(clusterId, params), signal)
      .then(json => {
        const items = json?.items || [];

        return {
          agents: items.map(makeUserGroup),
          startKey: json?.startKey,
          totalCount: json?.totalCount,
        };
      });
  }
}

const userGroupService = new UserGroupService();

export default userGroupService;
