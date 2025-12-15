/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { useQueries, UseQueryResult } from '@tanstack/react-query';
import { useCallback } from 'react';

import { AccessRequest } from 'gen-proto-ts/teleport/lib/teleterm/v1/access_request_pb';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useStoreSelector } from 'teleterm/ui/hooks/useStoreSelector';
import { RootClusterUri } from 'teleterm/ui/uri';

/**
 * Keeps request details for assumed request IDs.
 * Once loaded from the server, the details are cached indefinitely.
 */
export const useAssumedRequests = (
  uri: RootClusterUri
): Map<string, UseQueryResult<AccessRequest>> => {
  const appContext = useAppContext();
  const cluster = useStoreSelector(
    'clustersService',
    useCallback(state => state.clusters.get(uri), [uri])
  );

  const activeRequests = cluster?.loggedInUser.activeRequests || [];

  return useQueries({
    queries: activeRequests.map(id => ({
      // Only run this background query when the cluster is connected.
      enabled: cluster?.connected,
      queryKey: ['assumedAccessRequests', uri, id],
      // Fetch once and always use cached value.
      staleTime: Infinity,
      queryFn: async () => {
        const { response } = await appContext.tshd.getAccessRequest({
          clusterUri: uri,
          accessRequestId: id,
        });
        return response.request;
      },
      // Attach ID for the map below.
      meta: { id },
    })),
    combine: results => {
      return results.reduce((map, data, index) => {
        const id = activeRequests[index];
        map.set(id, data);
        return map;
      }, new Map<string, UseQueryResult<AccessRequest>>());
    },
  });
};
