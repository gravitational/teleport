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

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { WorkspaceContextProvider } from 'teleterm/ui/Documents';
import { RootClusterUri } from 'teleterm/ui/uri';

export const MockWorkspaceContextProvider: React.FC<
  PropsWithChildren<{
    rootClusterUri?: RootClusterUri;
  }>
> = props => {
  const appContext = useAppContext();

  const rootClusterUri =
    props.rootClusterUri || appContext.workspacesService.getRootClusterUri();

  return (
    <WorkspaceContextProvider
      value={{
        accessRequestsService:
          appContext.workspacesService.getWorkspaceAccessRequestsService(
            rootClusterUri
          ),
        documentsService:
          appContext.workspacesService.getWorkspaceDocumentService(
            rootClusterUri
          ),
        localClusterUri: rootClusterUri,
        rootClusterUri,
      }}
    >
      {props.children}
    </WorkspaceContextProvider>
  );
};
