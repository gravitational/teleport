/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { useCallback } from 'react';

import { useAppContext } from 'teleterm/ui/appContextProvider';

/**
 * useSearch returns a function which searches for the given list of space-separated keywords across
 * all root and leaf clusters that the user is currently logged in to.
 *
 * It does so by issuing a separate request for each resource type to each cluster. It fails if any
 * of those requests fail.
 */
export function useSearch() {
  const { clustersService, resourcesService } = useAppContext();
  clustersService.useState();

  return useCallback(
    async (search: string) => {
      const connectedClusters = clustersService
        .getClusters()
        .filter(c => c.connected);
      const searchPromises = connectedClusters.map(cluster =>
        resourcesService.searchResources(cluster.uri, search)
      );

      return (await Promise.all(searchPromises)).flat();
    },
    [clustersService, resourcesService]
  );
}
