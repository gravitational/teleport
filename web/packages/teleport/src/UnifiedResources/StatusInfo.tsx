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
  } = useResourceServersFetch<SharedResourceServer>({
    fetchFunc: async (params, signal) => {
      if (resource.kind === 'db') {
        const response = await fetchDatabaseServers({
          clusterId,
          params: {
            ...params,
            query: `name == "${resource.name}"`,
            searchAsRoles: resource.requiresRequest ? 'yes' : '',
          },
          signal,
        });
        const servers: DatabaseServer[] = response.agents.map(d => ({
          kind: 'db_server',
          ...d,
        }));
        return {
          agents: servers,
          startKey: response.startKey,
        };
      }
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
