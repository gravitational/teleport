/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';

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

export const WorkspaceContextProvider: React.FC<{
  value: {
    rootClusterUri: RootClusterUri;
    localClusterUri: ClusterUri;
    documentsService: DocumentsService;
    accessRequestsService: AccessRequestsService;
  };
}> = props => {
  return <WorkspaceContext.Provider {...props} />;
};

export const useWorkspaceContext = () => {
  const ctx = useAppContext();
  ctx.workspacesService.useState();

  return React.useContext(WorkspaceContext);
};
