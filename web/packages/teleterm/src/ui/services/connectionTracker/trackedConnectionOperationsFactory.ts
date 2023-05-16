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

import { LeafClusterUri, RootClusterUri, routing } from 'teleterm/ui/uri';
import { ClustersService } from 'teleterm/ui/services/clusters';
import {
  DocumentOrigin,
  WorkspacesService,
} from 'teleterm/ui/services/workspacesService';

import {
  getGatewayDocumentByConnection,
  getKubeDocumentByConnection,
  getServerDocumentByConnection,
} from './trackedConnectionUtils';
import {
  TrackedConnection,
  TrackedGatewayConnection,
  TrackedKubeConnection,
  TrackedServerConnection,
} from './types';

export class TrackedConnectionOperationsFactory {
  constructor(
    private _clustersService: ClustersService,
    private _workspacesService: WorkspacesService
  ) {}

  create(connection: TrackedConnection): TrackedConnectionOperations {
    switch (connection.kind) {
      case 'connection.server':
        return this.getConnectionServerOperations(connection);
      case 'connection.gateway':
        return this.getConnectionGatewayOperations(connection);
      case 'connection.kube':
        return this.getConnectionKubeOperations(connection);
    }
  }

  private getConnectionServerOperations(
    connection: TrackedServerConnection
  ): TrackedConnectionOperations {
    const { rootClusterId, leafClusterId } = routing.parseServerUri(
      connection.serverUri
    ).params;
    const { rootClusterUri, leafClusterUri } = this.getClusterUris({
      rootClusterId,
      leafClusterId,
    });

    const documentsService =
      this._workspacesService.getWorkspaceDocumentService(rootClusterUri);

    return {
      rootClusterUri,
      leafClusterUri,
      activate: params => {
        let srvDoc = documentsService
          .getDocuments()
          .find(getServerDocumentByConnection(connection));

        if (!srvDoc) {
          srvDoc = documentsService.createTshNodeDocument(
            connection.serverUri,
            params
          );
          srvDoc.status = 'connecting';
          srvDoc.login = connection.login;
          srvDoc.title = connection.title;

          documentsService.add(srvDoc);
        }
        documentsService.open(srvDoc.uri);
      },
      disconnect: async () => {
        documentsService
          .getDocuments()
          .filter(getServerDocumentByConnection(connection))
          .forEach(document => {
            documentsService.close(document.uri);
          });
      },
      remove: async () => {},
    };
  }

  private getConnectionGatewayOperations(
    connection: TrackedGatewayConnection
  ): TrackedConnectionOperations {
    const { rootClusterId, leafClusterId } = routing.parseDbUri(
      connection.targetUri
    ).params;
    const { rootClusterUri, leafClusterUri } = this.getClusterUris({
      rootClusterId,
      leafClusterId,
    });

    const documentsService =
      this._workspacesService.getWorkspaceDocumentService(rootClusterUri);

    return {
      rootClusterUri,
      leafClusterUri,
      activate: params => {
        let gwDoc = documentsService
          .getDocuments()
          .find(getGatewayDocumentByConnection(connection));

        if (!gwDoc) {
          gwDoc = documentsService.createGatewayDocument({
            targetUri: connection.targetUri,
            targetName: connection.targetName,
            targetUser: connection.targetUser,
            targetSubresourceName: connection.targetSubresourceName,
            title: connection.title,
            gatewayUri: connection.gatewayUri,
            port: connection.port,
            origin: params.origin,
          });

          documentsService.add(gwDoc);
        }
        documentsService.open(gwDoc.uri);
      },
      disconnect: () => {
        return this._clustersService
          .removeGateway(connection.gatewayUri)
          .then(() => {
            documentsService
              .getDocuments()
              .filter(getGatewayDocumentByConnection(connection))
              .forEach(document => {
                documentsService.close(document.uri);
              });
          });
      },
      remove: async () => {},
    };
  }

  private getConnectionKubeOperations(
    connection: TrackedKubeConnection
  ): TrackedConnectionOperations {
    const { rootClusterId, leafClusterId } = routing.parseKubeUri(
      connection.kubeUri
    ).params;
    const { rootClusterUri, leafClusterUri } = this.getClusterUris({
      rootClusterId,
      leafClusterId,
    });

    const documentsService =
      this._workspacesService.getWorkspaceDocumentService(rootClusterUri);

    return {
      rootClusterUri,
      leafClusterUri,
      activate: params => {
        let kubeConn = documentsService
          .getDocuments()
          .find(getKubeDocumentByConnection(connection));

        if (!kubeConn) {
          kubeConn = documentsService.createTshKubeDocument({
            kubeUri: connection.kubeUri,
            kubeConfigRelativePath: connection.kubeConfigRelativePath,
            origin: params.origin,
          });

          documentsService.add(kubeConn);
        }
        documentsService.open(kubeConn.uri);
      },
      disconnect: async () => {
        documentsService
          .getDocuments()
          .filter(getKubeDocumentByConnection(connection))
          .forEach(document => {
            documentsService.close(document.uri);
          });
      },
      remove: () => {
        return this._clustersService.removeKubeConfig(
          connection.kubeConfigRelativePath
        );
      },
    };
  }

  private getClusterUris({ rootClusterId, leafClusterId }) {
    const rootClusterUri = routing.getClusterUri({
      rootClusterId,
    });
    const leafClusterUri = routing.getClusterUri({
      rootClusterId,
      leafClusterId,
    });

    return {
      rootClusterUri: rootClusterUri as RootClusterUri,
      leafClusterUri:
        rootClusterUri === leafClusterUri
          ? undefined
          : (leafClusterUri as LeafClusterUri),
    };
  }
}

interface TrackedConnectionOperations {
  rootClusterUri: RootClusterUri;
  leafClusterUri: LeafClusterUri;

  activate(params: { origin: DocumentOrigin }): void;

  disconnect(): Promise<void>;

  remove(): Promise<void>;
}
