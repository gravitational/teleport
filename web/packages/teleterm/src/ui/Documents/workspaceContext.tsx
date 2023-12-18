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

import React, { PropsWithChildren } from 'react';

import { DocumentsService } from 'teleterm/ui/services/workspacesService';
import { AccessRequestsService } from 'teleterm/ui/services/workspacesService/accessRequestsService';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { ClusterUri, RootClusterUri } from 'teleterm/ui/uri';

const WorkspaceContext = React.createContext<{
  rootClusterUri: RootClusterUri;
  localClusterUri: ClusterUri;
  documentsService: DocumentsService;
  accessRequestsService: AccessRequestsService;
}>(null);

export const WorkspaceContextProvider: React.FC<
  PropsWithChildren<{
    value: {
      rootClusterUri: RootClusterUri;
      localClusterUri: ClusterUri;
      documentsService: DocumentsService;
      accessRequestsService: AccessRequestsService;
    };
  }>
> = props => {
  return <WorkspaceContext.Provider {...props} />;
};

export const useWorkspaceContext = () => {
  const ctx = useAppContext();
  ctx.workspacesService.useState();

  return React.useContext(WorkspaceContext);
};
