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

import { useStore } from 'shared/libs/stores';

import { ClustersService } from 'teleterm/ui/services/clusters';
import {
  Document,
  WorkspacesService,
} from 'teleterm/ui/services/workspacesService';
import { StatePersistenceService } from 'teleterm/ui/services/statePersistence';

import { getClusterName } from 'teleterm/ui/utils';

import { routing } from 'teleterm/ui/uri';

import { ImmutableStore } from '../immutableStore';

import { TrackedConnectionOperationsFactory } from './trackedConnectionOperationsFactory';
import {
  createGatewayConnection,
  createServerConnection,
  getGatewayConnectionByDocument,
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
    return this.state.connections.map(connection => {
      const { rootClusterUri, leafClusterUri } =
        this._trackedConnectionOperationsFactory.create(connection);
      const clusterUri = leafClusterUri || rootClusterUri;
      const clusterName =
        getClusterName(this._clusterService.findCluster(clusterUri)) ||
        routing.parseClusterName(clusterUri);
      return { ...connection, clusterName };
    });
  }

  async activateItem(id: string): Promise<void> {
    const connection = this.state.connections.find(c => c.id === id);
    const { rootClusterUri, activate } =
      this._trackedConnectionOperationsFactory.create(connection);

    if (rootClusterUri !== this._workspacesService.getRootClusterUri()) {
      await this._workspacesService.setActiveWorkspace(rootClusterUri);
    }
    activate();
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
    const { disconnect } =
      this._trackedConnectionOperationsFactory.create(connection);
    return disconnect();
  }

  removeItem(id: string): void {
    this.setState(draft => {
      draft.connections = draft.connections.filter(i => i.id !== id);
    });

    this._statePersistenceService.saveConnectionTrackerState(this.state);
  }

  removeItemsBelongingToRootCluster(clusterUri: string): void {
    this.setState(draft => {
      draft.connections = draft.connections.filter(i => {
        const { rootClusterUri } =
          this._trackedConnectionOperationsFactory.create(i);
        return rootClusterUri !== clusterUri;
      });
    });
  }

  dispose(): void {
    this._workspacesService.unsubscribe(this._refreshState);
    this._clusterService.unsubscribe(this._refreshState);
  }

  private _refreshState = () => {
    this.setState(draft => {
      // assign default "connected" values
      draft.connections.forEach(i => {
        if (i.kind === 'connection.gateway') {
          i.connected = !!this._clusterService.findGateway(i.gatewayUri);
        } else {
          i.connected = false;
        }
      });

      const docs = Array.from(
        Object.keys(this._workspacesService.getWorkspaces())
      )
        .flatMap(clusterUri => {
          const docService =
            this._workspacesService.getWorkspaceDocumentService(clusterUri);
          return docService?.getDocuments();
        })
        .filter(Boolean)
        .filter(
          d => d.kind === 'doc.gateway' || d.kind === 'doc.terminal_tsh_node'
        );

      if (!docs) {
        return;
      }

      while (docs.length > 0) {
        const doc = docs.pop();

        switch (doc.kind) {
          // process gateway connections
          case 'doc.gateway':
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
              gwConn.gatewayUri = doc.gatewayUri;
              gwConn.targetSubresourceName = doc.targetSubresourceName;
              gwConn.port = doc.port;
              gwConn.connected = !!this._clusterService.findGateway(
                doc.gatewayUri
              );
            }

            break;
          // process tsh connections
          case 'doc.terminal_tsh_node':
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
