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

import { useMemo } from 'react';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import {
  DisplayResults,
  isClusterSearchFilter,
  isResourceTypeSearchFilter,
  SearchFilter,
} from 'teleterm/ui/Search/searchResult';
import { useWorkspaceServiceState } from 'teleterm/ui/services/workspacesService';

export function useDisplayResults(args: {
  filters: SearchFilter[];
  inputValue: string;
}): DisplayResults {
  const { workspacesService } = useAppContext();
  useWorkspaceServiceState();

  const localClusterUri =
    workspacesService.getActiveWorkspace()?.localClusterUri;

  const activeDocument = workspacesService
    .getActiveWorkspaceDocumentService()
    ?.getActive();

  return useMemo(() => {
    const clusterDocument =
      activeDocument?.kind === 'doc.cluster' && activeDocument;
    const clusterFilter = args.filters.find(isClusterSearchFilter);
    const kinds = args.filters.filter(isResourceTypeSearchFilter);

    const shouldOpenInCurrentTab =
      clusterDocument &&
      (!clusterFilter ||
        clusterFilter.clusterUri === clusterDocument.clusterUri);

    return {
      kind: 'display-results',
      value: args.inputValue,
      documentUri: shouldOpenInCurrentTab ? clusterDocument.uri : undefined,
      clusterUri: shouldOpenInCurrentTab
        ? clusterDocument.clusterUri
        : clusterFilter?.clusterUri || localClusterUri,
      resourceKinds: kinds.map(kind => kind.resourceType),
    };
  }, [activeDocument, args.filters, args.inputValue, localClusterUri]);
}
