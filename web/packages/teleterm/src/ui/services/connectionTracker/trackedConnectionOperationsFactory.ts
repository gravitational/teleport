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

import { ClustersService } from 'teleterm/ui/services/clusters';
import {
  createDesktopSessionDocument,
  DocumentGateway,
  DocumentOrigin,
  WorkspacesService,
} from 'teleterm/ui/services/workspacesService';
import { LeafClusterUri, RootClusterUri, routing } from 'teleterm/ui/uri';

import {
  getDesktopDocumentByConnection,
  getGatewayDocumentByConnection,
  getGatewayKubeDocumentByConnection,
  getServerDocumentByConnection,
} from './trackedConnectionUtils';
import {
  TrackedConnection,
  TrackedDesktopConnection,
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
        return this.getConnectionGatewayKubeOperations(connection);
      case 'connection.desktop':
        return this.getConnectionDesktopOperations(connection);
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
    const { rootClusterId, leafClusterId } = routing.parseClusterUri(
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
          .find(getGatewayDocumentByConnection(connection)) as DocumentGateway;

        if (!gwDoc) {
          const gw = this._clustersService.findGatewayByConnectionParams({
            targetUri: connection.targetUri,
            targetUser: connection.targetUser,
            targetSubresourceName: connection.targetSubresourceName,
          });

          gwDoc = documentsService.createGatewayDocument({
            targetUri: connection.targetUri,
            targetName: connection.targetName,
            targetUser: connection.targetUser,
            targetSubresourceName: connection.targetSubresourceName,
            title: connection.title,
            // If the doc was closed but the gateway is still running, it's important for the
            // doc to reopen with the existing gateway URI. Otherwise the doc would attempt to
            // create a new gateway with the same connection params.
            gatewayUri: gw?.uri,
            port: connection.port,
            origin: params.origin,
          });

          documentsService.add(gwDoc);
        }
        documentsService.open(gwDoc.uri);
      },
      disconnect: async () => {
        // When disconnecting, assume that the gateway exists. If a gateway doesn't exist, the UI is
        // supposed to expose the remove operation, not the disconnect operation.
        const gw = this._clustersService.findGatewayByConnectionParams({
          targetUri: connection.targetUri,
          targetUser: connection.targetUser,
          targetSubresourceName: connection.targetSubresourceName,
        });

        return this._clustersService.removeGateway(gw.uri).then(() => {
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

  private getConnectionGatewayKubeOperations(
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
        let gwDoc = documentsService
          .getDocuments()
          .find(getGatewayKubeDocumentByConnection(connection));

        if (!gwDoc) {
          gwDoc = documentsService.createGatewayKubeDocument({
            targetUri: connection.kubeUri,
            origin: params.origin,
          });
          documentsService.add(gwDoc);
        }
        documentsService.open(gwDoc.uri);
      },
      disconnect: async () => {
        return (
          this._clustersService
            // We have to use `removeKubeGateway` instead of `removeGateway`,
            // because we need to support both the old kube connections
            // (which don't have gatewayUri and an underlying gateway)
            // and new ones (which do have a gateway).
            .removeKubeGateway(connection.kubeUri)
            .then(() => {
              documentsService
                .getDocuments()
                .filter(getGatewayKubeDocumentByConnection(connection))
                .forEach(document => {
                  documentsService.close(document.uri);
                });
            })
        );
      },
      remove: async () => {},
    };
  }

  private getConnectionDesktopOperations(
    connection: TrackedDesktopConnection
  ): TrackedConnectionOperations {
    const { rootClusterId, leafClusterId } = routing.parseClusterUri(
      connection.desktopUri
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
        let doc = documentsService
          .getDocuments()
          .find(getDesktopDocumentByConnection(connection));

        if (!doc) {
          doc = createDesktopSessionDocument({
            desktopUri: connection.desktopUri,
            login: connection.login,
            origin: params.origin,
          });
          documentsService.add(doc);
        }
        documentsService.open(doc.uri);
      },
      disconnect: async () => {
        documentsService
          .getDocuments()
          .filter(getDesktopDocumentByConnection(connection))
          .forEach(document => {
            documentsService.close(document.uri);
          });
      },
      remove: async () => {},
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
