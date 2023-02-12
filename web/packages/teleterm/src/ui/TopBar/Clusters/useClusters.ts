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

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { ClusterUri } from 'teleterm/ui/uri';

export function useClusters() {
  const { workspacesService, clustersService, commandLauncher } =
    useAppContext();

  workspacesService.useState();
  clustersService.useState();

  function findLeaves(clusterUri: string) {
    return clustersService
      .getClusters()
      .filter(c => c.leaf && c.uri.startsWith(clusterUri));
  }

  function hasPendingAccessRequest() {
    const accessRequestsService =
      workspacesService.getActiveWorkspaceAccessRequestsService();
    if (!accessRequestsService) {
      return false;
    }

    const pendingAccessRequest =
      accessRequestsService.getPendingAccessRequest();

    if (!pendingAccessRequest) {
      return false;
    }

    const count = accessRequestsService.getAddedResourceCount();
    return count > 0;
  }

  function clearPendingAccessRequest() {
    const accessRequestsService =
      workspacesService.getActiveWorkspaceAccessRequestsService();

    accessRequestsService?.clearPendingAccessRequest();
  }

  const rootClusterUri = workspacesService.getRootClusterUri();
  const localClusterUri =
    workspacesService.getActiveWorkspace()?.localClusterUri;
  const rootCluster = clustersService.findCluster(rootClusterUri);
  const items =
    (rootCluster && [rootCluster, ...findLeaves(rootClusterUri)]) || [];

  return {
    hasLeaves: items.some(i => i.leaf),
    hasPendingAccessRequest: hasPendingAccessRequest(),
    clearPendingAccessRequest,
    selectedItem:
      localClusterUri && clustersService.findCluster(localClusterUri),
    selectItem: (localClusterUri: ClusterUri) => {
      workspacesService.setWorkspaceLocalClusterUri(
        rootClusterUri,
        localClusterUri
      );
      commandLauncher.executeCommand('cluster-open', {
        clusterUri: localClusterUri,
      });
    },
    items,
  };
}
