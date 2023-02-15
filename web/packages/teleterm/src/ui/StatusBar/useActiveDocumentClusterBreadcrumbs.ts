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
import { getResourceUri } from 'teleterm/ui/services/workspacesService';
import { routing } from 'teleterm/ui/uri';

export function useActiveDocumentClusterBreadcrumbs(): string {
  const ctx = useAppContext();
  ctx.workspacesService.useState();
  ctx.clustersService.useState();

  const activeDocument = ctx.workspacesService
    .getActiveWorkspaceDocumentService()
    ?.getActive();

  if (!activeDocument) {
    return;
  }

  const resourceUri = getResourceUri(activeDocument);
  if (!resourceUri) {
    return;
  }

  const clusterUri = routing.ensureClusterUri(resourceUri);
  const rootClusterUri = routing.ensureRootClusterUri(resourceUri);

  const rootCluster = ctx.clustersService.findCluster(rootClusterUri);
  const leafCluster =
    clusterUri === rootClusterUri
      ? undefined
      : ctx.clustersService.findCluster(clusterUri);

  return [rootCluster, leafCluster]
    .filter(Boolean)
    .map(c => c.name)
    .join(' > ');
}
