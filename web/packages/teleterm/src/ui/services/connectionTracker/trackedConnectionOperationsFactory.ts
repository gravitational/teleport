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
    const clusterUri = routing.getClusterUri({
      rootClusterId: routing.parseServerUri(connection.serverUri).params
        .rootClusterId,
    });

    const documentsService =
      this._workspacesService.getWorkspaceDocumentService(clusterUri);

    return {
      clusterUri,
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
    const clusterUri = routing.getClusterUri({
      rootClusterId: routing.parseDbUri(connection.targetUri).params
        .rootClusterId,
    });

    const documentsService =
      this._workspacesService.getWorkspaceDocumentService(clusterUri);

    return {
      clusterUri,
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
}

interface TrackedConnectionOperations {
  clusterUri: string;

  activate(): void;

  disconnect(): Promise<void>;
}
