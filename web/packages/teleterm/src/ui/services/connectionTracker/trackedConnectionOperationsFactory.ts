import { routing } from 'teleterm/ui/uri';
import { ClustersService } from 'teleterm/ui/services/clusters';
import { WorkspacesService } from 'teleterm/ui/services/workspacesService';
import {
  getGatewayDocumentByConnection,
  getServerDocumentByConnection,
} from './trackedConnectionUtils';
import {
  TrackedConnection,
  TrackedGatewayConnection,
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
      activate: () => {
        let srvDoc = documentsService
          .getDocuments()
          .find(getServerDocumentByConnection(connection));

        if (!srvDoc) {
          srvDoc = documentsService.createTshNodeDocument(connection.serverUri);
          srvDoc.status = 'disconnected';
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
      activate: () => {
        let gwDoc = documentsService
          .getDocuments()
          .find(getGatewayDocumentByConnection(connection));

        if (!gwDoc) {
          gwDoc = documentsService.createGatewayDocument({
            targetUri: connection.targetUri,
            targetUser: connection.targetUser,
            title: connection.title,
            gatewayUri: connection.gatewayUri,
            port: connection.port,
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
    };
  }

  private getClusterUris({
    rootClusterId,
    leafClusterId,
  }: {
    rootClusterId: string;
    leafClusterId: string;
  }): { rootClusterUri: string; leafClusterUri: string } {
    const rootClusterUri = routing.getClusterUri({
      rootClusterId,
    });
    const leafClusterUri = routing.getClusterUri({
      rootClusterId,
      leafClusterId,
    });

    return {
      rootClusterUri,
      leafClusterUri:
        rootClusterUri === leafClusterUri ? undefined : leafClusterUri,
    };
  }
}

interface TrackedConnectionOperations {
  rootClusterUri: string;
  leafClusterUri: string;

  activate(): void;

  disconnect(): Promise<void>;
}
