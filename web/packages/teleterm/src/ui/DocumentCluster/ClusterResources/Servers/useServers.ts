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
import { Server, ServerSideParams } from 'teleterm/services/tshd/types';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { makeServer } from 'teleterm/ui/services/clusters';

import { useServerSideResources } from '../useServerSideResources';

import type * as uri from 'teleterm/ui/uri';

export function useServers() {
  const appContext = useAppContext();

  const { fetchAttempt, ...serversideResources } =
    useServerSideResources<Server>(
      { fieldName: 'hostname', dir: 'ASC' }, // default sort
      (params: ServerSideParams) =>
        appContext.resourcesService.fetchServers(params)
    );

  function getSshLogins(serverUri: uri.ServerUri): string[] {
    const cluster = appContext.clustersService.findClusterByResource(serverUri);
    return cluster?.loggedInUser?.sshLoginsList || [];
  }

  function connect(server: ReturnType<typeof makeServer>, login: string): void {
    const rootCluster = appContext.clustersService.findRootClusterByResource(
      server.uri
    );
    const documentsService =
      appContext.workspacesService.getWorkspaceDocumentService(rootCluster.uri);
    const doc = documentsService.createTshNodeDocument(server.uri);
    doc.title = `${login}@${server.hostname}`;
    doc.login = login;

    documentsService.add(doc);
    documentsService.setLocation(doc.uri);
  }

  return {
    fetchAttempt,
    getSshLogins,
    connect,
    ...serversideResources,
  };
}

export type State = ReturnType<typeof useServers>;
