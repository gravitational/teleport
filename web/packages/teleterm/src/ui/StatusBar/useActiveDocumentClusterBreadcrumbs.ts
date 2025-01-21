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

import { ComponentType, useCallback } from 'react';

import { IconProps } from 'design/Icon/Icon';

import {
  getResourceUri,
  getStaticNameAndIcon,
} from 'teleterm/ui/services/workspacesService';
import { routing } from 'teleterm/ui/uri';

import { useStoreSelector } from '../hooks/useStoreSelector';

interface Breadcrumb {
  name: string;
  Icon?: ComponentType<IconProps>;
}

export function useActiveDocumentClusterBreadcrumbs(): Breadcrumb[] {
  const activeDocument = useStoreSelector(
    'workspacesService',
    useCallback(state => {
      const workspace = state.workspaces[state.rootClusterUri];
      return workspace?.documents.find(d => d.uri === workspace?.location);
    }, [])
  );
  const resourceUri = activeDocument && getResourceUri(activeDocument);
  const staticNameAndIcon =
    activeDocument && getStaticNameAndIcon(activeDocument);
  const clusterUri = resourceUri && routing.ensureClusterUri(resourceUri);
  const rootClusterUri =
    resourceUri && routing.ensureRootClusterUri(resourceUri);

  const cluster = useStoreSelector(
    'clustersService',
    useCallback(state => state.clusters.get(clusterUri), [clusterUri])
  );
  const rootCluster = useStoreSelector(
    'clustersService',
    useCallback(state => state.clusters.get(rootClusterUri), [rootClusterUri])
  );

  if (!cluster || !rootCluster || !staticNameAndIcon) {
    return;
  }

  return [
    { name: rootCluster.name },
    clusterUri !== rootClusterUri && { name: cluster.name },
    {
      name: staticNameAndIcon.name,
      Icon: staticNameAndIcon.Icon,
    },
  ].filter(Boolean);
}
