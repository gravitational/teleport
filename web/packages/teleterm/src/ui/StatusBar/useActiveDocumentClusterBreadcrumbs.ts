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
