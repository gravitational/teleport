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

import { useClusterContext } from 'teleterm/ui/DocumentCluster/clusterContext';
import { useAppContext } from 'teleterm/ui/appContextProvider';

export function useServers() {
  const appContext = useAppContext();
  const clusterContext = useClusterContext();
  const servers = clusterContext.getServers();
  const syncStatus = clusterContext.getSyncStatus().servers;

  function getSshLogins(serverUri: string): string[] {
    const cluster = appContext.clustersService.findClusterByResource(serverUri);
    return cluster?.loggedInUser?.sshLoginsList || [];
  }

  function connect(serverUri: string, login: string): void {
    const server = appContext.clustersService.getServer(serverUri);

    const rootCluster =
      appContext.clustersService.findRootClusterByResource(serverUri);
    const documentsService =
      appContext.workspacesService.getWorkspaceDocumentService(rootCluster.uri);
    const doc = documentsService.createTshNodeDocument(serverUri);
    doc.title = `${login}@${server.hostname}`;
    doc.login = login;

    documentsService.add(doc);
    documentsService.setLocation(doc.uri);
  }

  return {
    servers,
    syncStatus,
    getSshLogins,
    connect,
  };
}

export type State = ReturnType<typeof useServers>;
