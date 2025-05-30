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

import { useStore } from 'shared/libs/stores';

import { ClustersService } from 'teleterm/ui/services/clusters';
import { StatePersistenceService } from 'teleterm/ui/services/statePersistence';
import {
  Document,
  DocumentOrigin,
  WorkspacesService,
} from 'teleterm/ui/services/workspacesService';
import * as uri from 'teleterm/ui/uri';
import { RootClusterUri, routing } from 'teleterm/ui/uri';
import { assertUnreachable } from 'teleterm/ui/utils';

import { ImmutableStore } from '../immutableStore';
import { TrackedConnectionOperationsFactory } from './trackedConnectionOperationsFactory';
import {
  createDesktopConnection,
  createGatewayConnection,
  createGatewayKubeConnection,
  createServerConnection,
  getDesktopConnectionByDocument,
  getGatewayConnectionByDocument,
  getGatewayKubeConnectionByDocument,
  getServerConnectionByDocument,
} from './trackedConnectionUtils';
import {
  ExtendedTrackedConnection,
  TrackedConnection,
  TrackedGatewayConnection,
} from './types';

export class ConnectionTrackerService extends ImmutableStore<ConnectionTrackerState> {
  private _trackedConnectionOperationsFactory: TrackedConnectionOperationsFactory;
  state: ConnectionTrackerState = {
    connections: [],
  };

  constructor(
    private _statePersistenceService: StatePersistenceService,
    private _workspacesService: WorkspacesService,
    private _clusterService: ClustersService
  ) {
    super();

    this.state.connections = this._restoreConnectionItems();
    this._workspacesService.subscribe(this._refreshState);
    this._clusterService.subscribe(this._refreshState);
    this._trackedConnectionOperationsFactory =
      new TrackedConnectionOperationsFactory(
        this._clusterService,
        this._workspacesService
      );
  }

  useState() {
    return useStore(this).state;
  }

  getConnections(): ExtendedTrackedConnection[] {
    return this.state.connections
      .map(connection => {
        const trackedConnection =
          this._trackedConnectionOperationsFactory.create(connection);
        // A connection is undefined when the state read from the disk
        // contains a connection not supported by the given Connect version.
        //
        // For example, the user can open a desktop connection in Connect v18
        // and then downgrade to a version that doesn't support desktops.
        // That connection should be shown as 'UNKNOWN' in the connection list.
        if (!trackedConnection) {
          return;
        }
        const { rootClusterUri, leafClusterUri } = trackedConnection;
        const clusterUri = leafClusterUri || rootClusterUri;
        const clusterName =
          this._clusterService.findCluster(clusterUri)?.name ||
          routing.parseClusterName(clusterUri);
        return { ...connection, clusterName };
      })
      .filter(Boolean);
  }

  async activateItem(
    id: string,
    params: { origin: DocumentOrigin }
  ): Promise<void> {
    const connection = this.state.connections.find(c => c.id === id);
    const { rootClusterUri, activate } =
      this._trackedConnectionOperationsFactory.create(connection);

    if (rootClusterUri !== this._workspacesService.getRootClusterUri()) {
      await this._workspacesService.setActiveWorkspace(rootClusterUri);
    }
    activate(params);
  }

  findConnection(id: string): TrackedConnection | undefined {
    return this.state.connections.find(c => c.id === id);
  }

  findConnectionByDocument(document: Document): TrackedConnection {
    switch (document.kind) {
      case 'doc.terminal_tsh_node':
        return this.state.connections.find(
          getServerConnectionByDocument(document)
        );
      case 'doc.gateway':
        return this.state.connections.find(
          getGatewayConnectionByDocument(document)
        );
      case 'doc.gateway_kube':
        return this.state.connections.find(
          getGatewayKubeConnectionByDocument(document)
        );
      case 'doc.desktop_session':
        return this.state.connections.find(
          getDesktopConnectionByDocument(document)
        );
    }
  }

  setState(
    nextState: (
      draftState: ConnectionTrackerState
    ) => ConnectionTrackerState | void
  ): void {
    super.setState(nextState);
    this._statePersistenceService.saveConnectionTrackerState(this.state);
  }

  async disconnectItem(id: string): Promise<void> {
    const connection = this.state.connections.find(c => c.id === id);
    if (!connection) {
      return;
    }

    return this._trackedConnectionOperationsFactory
      .create(connection)
      .disconnect();
  }

  async removeItem(id: string): Promise<void> {
    const connection = this.state.connections.find(c => c.id === id);
    if (!connection) {
      return;
    }

    await this._trackedConnectionOperationsFactory.create(connection).remove();

    this.setState(draft => {
      draft.connections = draft.connections.filter(i => i.id !== id);
    });
  }

  removeItemsBelongingToRootCluster(clusterUri: uri.RootClusterUri): void {
    this.setState(draft => {
      draft.connections = draft.connections.filter(i => {
        const { rootClusterUri } =
          this._trackedConnectionOperationsFactory.create(i);
        return rootClusterUri !== clusterUri;
      });
    });
  }

  async disconnectAndRemoveItemsBelongingToResource(
    resourceUri: uri.ResourceUri
  ): Promise<void> {
    const connections = this.getConnections().filter(s => {
      switch (s.kind) {
        case 'connection.server':
          return s.serverUri === resourceUri;
        case 'connection.gateway':
          return s.targetUri === resourceUri;
        case 'connection.kube':
          return s.kubeUri === resourceUri;
        case 'connection.desktop':
          return s.desktopUri === resourceUri;
        default:
          return assertUnreachable(s);
      }
    });
    await Promise.all([
      connections.map(async connection => {
        await this.disconnectItem(connection.id);
        await this.removeItem(connection.id);
      }),
    ]);
  }

  dispose(): void {
    this._workspacesService.unsubscribe(this._refreshState);
    this._clusterService.unsubscribe(this._refreshState);
  }

  private _refreshState = () => {
    this.setState(draft => {
      // assign default "connected" values
      draft.connections.forEach(i => {
        switch (i.kind) {
          case 'connection.gateway': {
            i.connected = !!this._clusterService.findGatewayByConnectionParams({
              targetUri: i.targetUri,
              targetUser: i.targetUser,
              targetSubresourceName: i.targetSubresourceName,
            });
            break;
          }
          case 'connection.kube': {
            i.connected = !!this._clusterService.findGatewayByConnectionParams({
              targetUri: i.kubeUri,
            });
            break;
          }
          default: {
            i.connected = false;
            break;
          }
        }
      });

      const docs = Array.from(
        Object.keys(this._workspacesService.getWorkspaces())
      )
        .flatMap(clusterUri => {
          const docService =
            this._workspacesService.getWorkspaceDocumentService(
              clusterUri as RootClusterUri
            );
          return docService?.getDocuments();
        })
        .filter(Boolean)
        .filter(
          d =>
            d.kind === 'doc.gateway' ||
            d.kind === 'doc.gateway_kube' ||
            d.kind === 'doc.terminal_tsh_node' ||
            d.kind === 'doc.desktop_session'
        );

      if (!docs) {
        return;
      }

      while (docs.length > 0) {
        const doc = docs.pop();

        switch (doc.kind) {
          // process gateway connections
          case 'doc.gateway': {
            // Ignore freshly created docs which have no corresponding gateway yet.
            if (!doc.port) {
              break;
            }

            const gwConn = draft.connections.find(
              getGatewayConnectionByDocument(doc)
            ) as TrackedGatewayConnection;

            if (!gwConn) {
              const newItem = createGatewayConnection(doc);
              draft.connections.push(newItem);
            } else {
              // In case the document changes, update the gateway title.
              // Specifically, it addresses a case where we changed a title format
              // for db gateway documents, and we wanted this change to be reflected
              // in already created connections.
              gwConn.title = doc.title;
              gwConn.targetSubresourceName = doc.targetSubresourceName;
              gwConn.port = doc.port;
              gwConn.connected = !!this._clusterService.findGateway(
                doc.gatewayUri
              );
            }
            break;
          }
          // process kube gateway connections
          case 'doc.gateway_kube': {
            const kubeConn = draft.connections.find(
              getGatewayKubeConnectionByDocument(doc)
            );

            if (kubeConn) {
              kubeConn.connected =
                !!this._clusterService.findGatewayByConnectionParams({
                  targetUri: doc.targetUri,
                });
            } else {
              const newItem = createGatewayKubeConnection(doc);
              draft.connections.push(newItem);
            }
            break;
          }
          // process tsh connections
          case 'doc.terminal_tsh_node': {
            const tshConn = draft.connections.find(
              getServerConnectionByDocument(doc)
            );

            if (tshConn) {
              tshConn.connected = doc.status === 'connected';
            } else {
              const newItem = createServerConnection(doc);
              draft.connections.push(newItem);
            }
            break;
          }
          case 'doc.desktop_session': {
            const desktopConn = draft.connections.find(
              getDesktopConnectionByDocument(doc)
            );

            if (desktopConn) {
              desktopConn.connected = doc.status === 'connected';
            } else {
              const newItem = createDesktopConnection(doc);
              draft.connections.push(newItem);
            }
            break;
          }
        }
      }
    });
  };

  private _restoreConnectionItems(): TrackedConnection[] {
    const savedState =
      this._statePersistenceService.getConnectionTrackerState();
    if (savedState && Array.isArray(savedState.connections)) {
      // restored connections cannot have connected state
      savedState.connections.forEach(i => {
        i.connected = false;
      });

      return savedState.connections;
    }

    return [];
  }
}

export type ConnectionTrackerState = {
  connections: TrackedConnection[];
};
