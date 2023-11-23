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

import { useMemo } from 'react';

import {
  isClusterSearchFilter,
  isResourceTypeSearchFilter,
  SearchFilter,
  DisplayResults,
} from 'teleterm/ui/Search/searchResult';
import { useAppContext } from 'teleterm/ui/appContextProvider';

export function useDisplayResults(args: {
  filters: SearchFilter[];
  inputValue: string;
}): DisplayResults {
  const { workspacesService } = useAppContext();
  workspacesService.useState();

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
