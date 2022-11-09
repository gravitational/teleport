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

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useClusterContext } from 'teleterm/ui/DocumentCluster/clusterContext';
import { routing } from 'teleterm/ui/uri';
import { GatewayProtocol } from 'teleterm/ui/services/clusters';

export function useDatabases() {
  const appContext = useAppContext();
  const clusterContext = useClusterContext();
  const dbs = clusterContext.getDbs();
  const syncStatus = clusterContext.getSyncStatus().dbs;

  function connect(dbUri: string, dbUser: string): void {
    const db = appContext.clustersService.findDb(dbUri);
    const rootClusterUri = routing.ensureRootClusterUri(db.uri);
    const documentsService =
      appContext.workspacesService.getWorkspaceDocumentService(rootClusterUri);

    const doc = documentsService.createGatewayDocument({
      // Not passing the `gatewayUri` field here, as at this point the gateway doesn't exist yet.
      // `port` is not passed as well, we'll let the tsh daemon pick a random one.
      targetUri: db.uri,
      targetName: db.name,
      targetUser: getTargetUser(db.protocol as GatewayProtocol, dbUser),
    });

    const connectionToReuse =
      appContext.connectionTracker.findConnectionByDocument(doc);

    if (connectionToReuse) {
      appContext.connectionTracker.activateItem(connectionToReuse.id);
    } else {
      documentsService.add(doc);
      documentsService.open(doc.uri);
    }
  }

  function getTargetUser(
    protocol: GatewayProtocol,
    providedDbUser: string
  ): string {
    // we are replicating tsh behavior (user can be omitted for Redis)
    // https://github.com/gravitational/teleport/blob/796e37bdbc1cb6e0a93b07115ffefa0e6922c529/tool/tsh/db.go#L240-L244
    // but unlike tsh, Connect has to provide a user that is then used in a gateway document
    if (protocol === 'redis') {
      return providedDbUser || 'default';
    }

    return providedDbUser;
  }

  return {
    connect,
    dbs,
    syncStatus,
  };
}

export type State = ReturnType<typeof useDatabases>;
