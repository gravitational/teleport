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
  ResourceStatus,
  SharedResourceServer,
  UnifiedResourceDefinition,
  useResourceServersFetch,
} from 'shared/components/UnifiedResources';
import { UnhealthyStatusInfo } from 'shared/components/UnifiedResources/shared/StatusInfo';

import { ResourcesResponse } from 'teleport/services/agents';
import { cloneAbortSignal } from 'teleterm/services/tshd/cloneableClient';
import { useAppContext } from 'teleterm/ui/appContextProvider';

export function StatusInfo({
  resource,
  clusterUri,
}: {
  /**
   * the resource the user selected to look into the status of
   */
  resource: UnifiedResourceDefinition;
  clusterUri: string;
}) {
  const ctx = useAppContext();
  const {
    fetch: fetchResourceServers,
    resources: resourceServers,
    attempt: fetchResourceServersAttempt,
  } = useResourceServersFetch({
    fetchFunc: async (params, signal) => {
      let response: ResourcesResponse<SharedResourceServer>;

      if (resource.kind === 'db') {
        const { response: resp } = await ctx.tshd.listDatabaseServers(
          {
            ...params,
            clusterUri,
            useSearchAsRoles: resource.requiresRequest ? true : false,
            predicateExpression: `name == "${resource.name}"`,
          },
          { abort: cloneAbortSignal(signal) }
        );
        const servers: DatabaseServer[] = resp.servers.map(d => ({
          kind: 'db_server',
          hostname: d.hostname,
          hostId: d.hostId,
          targetHealth: d.targetHealth && {
            status: d.targetHealth.status as ResourceStatus,
            error: d.targetHealth.error,
          },
        }));
        response = {
          agents: servers,
          startKey: resp.nextKey,
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
