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

import {
  createContext,
  FC,
  PropsWithChildren,
  useCallback,
  useContext,
} from 'react';

import { useStoreSelector } from 'teleterm/ui/hooks/useStoreSelector';
import { DocumentsService } from 'teleterm/ui/services/workspacesService';
import { AccessRequestsService } from 'teleterm/ui/services/workspacesService/accessRequestsService';
import { ClusterUri, RootClusterUri } from 'teleterm/ui/uri';

const WorkspaceContext = createContext<{
  rootClusterUri: RootClusterUri;
  localClusterUri: ClusterUri;
  documentsService: DocumentsService;
  accessRequestsService: AccessRequestsService;
}>(null);

export const WorkspaceContextProvider: FC<
  PropsWithChildren<{
    value: {
      rootClusterUri: RootClusterUri;
      localClusterUri: ClusterUri;
      documentsService: DocumentsService;
      accessRequestsService: AccessRequestsService;
    };
  }>
> = props => {
  // Re-render the context provider whenever the state of the relevant workspace changes. The
  // context provider cannot re-render only when its props change.
  // For example, if a new document gets added, none of the props are going to change, but the
  // callsite that uses useWorkspaceContext might want to get re-rendered in this case, as
  // technically documentsService returned from useWorkspaceContext might return new state.
  useStoreSelector(
    'workspacesService',
    useCallback(
      state => state.workspaces[props.value.rootClusterUri],
      [props.value.rootClusterUri]
    )
  );
  return <WorkspaceContext.Provider {...props} />;
};

export const useWorkspaceContext = () => {
  const context = useContext(WorkspaceContext);

  if (!context) {
    throw new Error(
      'useWorkspaceContext must be used within a WorkspaceContextProvider'
    );
  }

  return context;
};
