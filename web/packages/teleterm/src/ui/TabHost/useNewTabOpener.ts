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

import { useCallback } from 'react';

import { DocumentsService } from 'teleterm/ui/services/workspacesService';
import { ClusterUri, routing } from 'teleterm/ui/uri';

export function useNewTabOpener({
  documentsService,
  localClusterUri,
}: {
  documentsService: DocumentsService;
  localClusterUri: ClusterUri;
}) {
  const openClusterTab = useCallback(() => {
    if (!localClusterUri) {
      return;
    }

    const clusterDocument = documentsService.createClusterDocument({
      clusterUri: localClusterUri,
    });

    documentsService.add(clusterDocument);
    documentsService.open(clusterDocument.uri);
  }, [documentsService, localClusterUri]);

  const openTerminalTab = useCallback(() => {
    if (!localClusterUri) {
      return;
    }

    const { params } = routing.parseClusterUri(localClusterUri);
    documentsService.openNewTerminal({
      rootClusterId: params.rootClusterId,
      leafClusterId: params.leafClusterId,
    });
  }, [documentsService, localClusterUri]);

  return { openClusterTab, openTerminalTab };
}
