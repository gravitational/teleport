/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import {
  DatabaseServer,
  SharedResourceServer,
  UnifiedResourceDefinition,
  useResourceServersFetch,
} from 'shared/components/UnifiedResources';
import { UnhealthyStatusInfo } from 'shared/components/UnifiedResources/shared/StatusInfo';

import { ResourcesResponse } from 'teleport/services/agents';
import { fetchDatabaseServers } from 'teleport/services/databases/databases';

export function StatusInfo({
  resource,
  clusterId,
}: {
  /**
   * the resource the user selected to look into the status of
   */
  resource: UnifiedResourceDefinition;
  clusterId: string;
}) {
  const {
    fetch: fetchResourceServers,
    resources: resourceServers,
    attempt: fetchResourceServersAttempt,
  } = useResourceServersFetch({
    fetchFunc: async (params, signal) => {
      let response: ResourcesResponse<SharedResourceServer>;

      if (resource.kind === 'db') {
        const resp = await fetchDatabaseServers({
          clusterId,
          params: {
            ...params,
            query: `name == "${resource.name}"`,
            searchAsRoles: resource.requiresRequest ? 'yes' : '',
          },
          signal,
        });
        const servers: DatabaseServer[] = resp.agents.map(d => ({
          kind: 'db_server',
          ...d,
        }));
        response = {
          agents: servers,
          startKey: resp.startKey,
        };
      }

      return response;
    },
  });

  return (
    <UnhealthyStatusInfo
      resource={resource}
      fetch={fetchResourceServers}
      servers={resourceServers}
      attempt={fetchResourceServersAttempt}
    />
  );
}
