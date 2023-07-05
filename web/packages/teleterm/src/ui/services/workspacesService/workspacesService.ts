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

import { useStore } from 'shared/libs/stores';
import { arrayObjectIsEqual } from 'shared/utils/highbar';

/* eslint-disable @typescript-eslint/ban-ts-comment*/
// @ts-ignore
import { ResourceKind } from 'e-teleport/Workflow/NewRequest/useNewRequest';

import { ModalsService } from 'teleterm/ui/services/modals';
import { ClustersService } from 'teleterm/ui/services/clusters';
import {
  StatePersistenceService,
  WorkspacesPersistedState,
} from 'teleterm/ui/services/statePersistence';
import { ImmutableStore } from 'teleterm/ui/services/immutableStore';
import { NotificationsService } from 'teleterm/ui/services/notifications';
import {
  ClusterOrResourceUri,
  ClusterUri,
  DocumentUri,
  RootClusterUri,
  routing,
} from 'teleterm/ui/uri';

import {
  AccessRequestsService,
  getEmptyPendingAccessRequest,
} from './accessRequestsService';

import { Document, DocumentsService } from './documentsService';

export interface WorkspacesState {
  rootClusterUri?: RootClusterUri;
  workspaces: Record<RootClusterUri, Workspace>;
}

export interface Workspace {
  localClusterUri: ClusterUri;
  documents: Document[];
  location: DocumentUri;
  accessRequests: {
    isBarCollapsed: boolean;
    pending: PendingAccessRequest;
  };
  previous?: {
    documents: Document[];
    location: DocumentUri;
  };
}

export class WorkspacesService extends ImmutableStore<WorkspacesState> {
  private documentsServicesCache = new Map<RootClusterUri, DocumentsService>();
  private accessRequestsServicesCache = new Map<
    RootClusterUri,
    AccessRequestsService
  >();
  state: WorkspacesState = {
    rootClusterUri: undefined,
    workspaces: {},
  };

  constructor(
    private modalsService: ModalsService,
    private clustersService: ClustersService,
    private notificationsService: NotificationsService,
    private statePersistenceService: StatePersistenceService
  ) {
    super();
  }

  getActiveWorkspace() {
    return this.state.workspaces[this.state.rootClusterUri];
  }

  getRootClusterUri() {
    return this.state.rootClusterUri;
  }

  getWorkspaces() {
    return this.state.workspaces;
  }

  getWorkspace(clusterUri: RootClusterUri) {
    return this.state.workspaces[clusterUri];
  }

  getActiveWorkspaceDocumentService() {
    if (!this.state.rootClusterUri) {
      return;
    }
    return this.getWorkspaceDocumentService(this.state.rootClusterUri);
  }

  getActiveWorkspaceAccessRequestsService() {
    if (!this.state.rootClusterUri) {
      return;
    }
    return this.getWorkspaceAccessRequestsService(this.state.rootClusterUri);
  }

  setWorkspaceLocalClusterUri(
    clusterUri: RootClusterUri,
    localClusterUri: ClusterUri
  ): void {
    this.setState(draftState => {
      draftState.workspaces[clusterUri].localClusterUri = localClusterUri;
    });
  }

  getWorkspaceDocumentService(
    clusterUri: RootClusterUri
  ): DocumentsService | undefined {
    if (!this.documentsServicesCache.has(clusterUri)) {
      this.documentsServicesCache.set(
        clusterUri,
        new DocumentsService(
          () => {
            return this.state.workspaces[clusterUri];
          },
          newState =>
            this.setState(draftState => {
              newState(draftState.workspaces[clusterUri]);
            })
        )
      );
    }

    return this.documentsServicesCache.get(clusterUri);
  }

  getWorkspaceAccessRequestsService(
    clusterUri: RootClusterUri
  ): AccessRequestsService | undefined {
    if (!this.accessRequestsServicesCache.has(clusterUri)) {
      this.accessRequestsServicesCache.set(
        clusterUri,
        new AccessRequestsService(
          () => {
            return this.state.workspaces[clusterUri].accessRequests;
          },
          newState =>
            this.setState(draftState => {
              newState(draftState.workspaces[clusterUri].accessRequests);
            })
        )
      );
    }
    return this.accessRequestsServicesCache.get(clusterUri);
  }

  isDocumentActive(documentUri: string): boolean {
    const documentService = this.getActiveWorkspaceDocumentService();
    return documentService && documentService.isActive(documentUri);
  }

  doesResourceBelongToActiveWorkspace(
    resourceUri: ClusterOrResourceUri
  ): boolean {
    return (
      this.state.rootClusterUri &&
      routing.belongsToProfile(this.state.rootClusterUri, resourceUri)
    );
  }

  useState() {
    return useStore(this);
  }

  setState(nextState: (draftState: WorkspacesState) => WorkspacesState | void) {
    super.setState(nextState);
    this.persistState();
  }

  setActiveWorkspace(clusterUri: RootClusterUri): Promise<void> {
    const setWorkspace = () => {
      this.setState(draftState => {
        // adding a new workspace
        if (!draftState.workspaces[clusterUri]) {
          draftState.workspaces[clusterUri] =
            this.getWorkspaceDefaultState(clusterUri);
        }
        draftState.rootClusterUri = clusterUri;
      });
    };

    // empty cluster URI - no cluster selected
    if (!clusterUri) {
      this.setState(draftState => {
        draftState.rootClusterUri = undefined;
      });
      return Promise.resolve();
    }

    const cluster = this.clustersService.findCluster(clusterUri);
    if (!cluster) {
      this.notificationsService.notifyError({
        title: 'Could not set cluster as active',
        description: `Cluster with URI ${clusterUri} does not exist`,
      });
      this.logger.warn(
        `Could not find cluster with uri ${clusterUri} when changing active cluster`
      );
      return Promise.resolve();
    }

    return new Promise<void>((resolve, reject) => {
      if (cluster.connected) {
        setWorkspace();
        return resolve();
      }
      this.modalsService.openClusterConnectDialog({
        clusterUri: clusterUri,
        onCancel: () => {
          reject();
        },
        onSuccess: () => {
          setWorkspace();
          resolve();
        },
      });
    })
      .then(() => {
        return new Promise<void>(resolve => {
          if (!this.getWorkspace(clusterUri)?.previous) {
            return resolve();
          }
          this.modalsService.openDocumentsReopenDialog({
            onConfirm: () => {
              this.reopenPreviousDocuments(clusterUri);
              resolve();
            },
            onCancel: () => {
              this.discardPreviousDocuments(clusterUri);
              resolve();
            },
          });
        });
      })
      .catch(() => undefined); // catch ClusterConnectDialog cancellation
  }

  removeWorkspace(clusterUri: RootClusterUri): void {
    this.setState(draftState => {
      delete draftState.workspaces[clusterUri];
    });
  }

  getConnectedWorkspacesClustersUri() {
    return (Object.keys(this.state.workspaces) as RootClusterUri[]).filter(
      clusterUri => this.clustersService.findCluster(clusterUri)?.connected
    );
  }

  restorePersistedState(): void {
    const persistedState = this.statePersistenceService.getWorkspacesState();
    const restoredWorkspaces = this.clustersService
      .getRootClusters()
      .reduce((workspaces, cluster) => {
        const persistedWorkspace = persistedState.workspaces[cluster.uri];
        const workspaceDefaultState = this.getWorkspaceDefaultState(
          persistedWorkspace?.localClusterUri || cluster.uri
        );
        const persistedWorkspaceDocuments = persistedWorkspace?.documents;

        workspaces[cluster.uri] = {
          ...workspaceDefaultState,
          previous: this.canReopenPreviousDocuments({
            previousDocuments: persistedWorkspaceDocuments,
            currentDocuments: workspaceDefaultState.documents,
          })
            ? {
                location: persistedWorkspace.location,
                documents: persistedWorkspaceDocuments,
              }
            : undefined,
        };
        return workspaces;
      }, {});

    this.setState(draftState => {
      draftState.workspaces = restoredWorkspaces;
    });

    if (persistedState.rootClusterUri) {
      this.setActiveWorkspace(persistedState.rootClusterUri);
    }
  }

  private reopenPreviousDocuments(clusterUri: RootClusterUri): void {
    this.setState(draftState => {
      const workspace = draftState.workspaces[clusterUri];
      workspace.documents = workspace.previous.documents.map(d => {
        //TODO: create a function that will prepare a new document, it will be used in:
        // DocumentsService
        // TrackedConnectionOperationsFactory
        // here
        if (
          d.kind === 'doc.terminal_tsh_kube' ||
          d.kind === 'doc.terminal_tsh_node'
        ) {
          return {
            ...d,
            status: 'connecting',
            origin: 'reopened_session',
          };
        }

        if (d.kind === 'doc.gateway') {
          return {
            ...d,
            origin: 'reopened_session',
          };
        }
        return d;
      });
      workspace.location = workspace.previous.location;
      workspace.previous = undefined;
    });
  }

  private discardPreviousDocuments(clusterUri: RootClusterUri): void {
    this.setState(draftState => {
      const workspace = draftState.workspaces[clusterUri];
      workspace.previous = undefined;
    });
  }

  private canReopenPreviousDocuments({
    previousDocuments,
    currentDocuments,
  }: {
    previousDocuments?: Document[];
    currentDocuments: Document[];
  }): boolean {
    const omitUriAndTitle = (documents: Document[]) =>
      documents.map(d => ({ ...d, uri: undefined, title: undefined }));

    return (
      previousDocuments?.length &&
      !arrayObjectIsEqual(
        omitUriAndTitle(previousDocuments),
        omitUriAndTitle(currentDocuments)
      )
    );
  }

  private getWorkspaceDefaultState(localClusterUri: ClusterUri): Workspace {
    const rootClusterUri = routing.ensureRootClusterUri(localClusterUri);
    const defaultDocument = this.getWorkspaceDocumentService(
      rootClusterUri
    ).createClusterDocument({ clusterUri: localClusterUri });
    return {
      accessRequests: {
        pending: getEmptyPendingAccessRequest(),
        isBarCollapsed: false,
      },
      localClusterUri,
      location: defaultDocument.uri,
      documents: [defaultDocument],
    };
  }

  private persistState(): void {
    const stateToSave: WorkspacesPersistedState = {
      rootClusterUri: this.state.rootClusterUri,
      workspaces: {},
    };
    for (let w in this.state.workspaces) {
      const workspace = this.state.workspaces[w];
      stateToSave.workspaces[w] = {
        localClusterUri: workspace.localClusterUri,
        location: workspace.previous?.location || workspace.location,
        documents: workspace.previous?.documents || workspace.documents,
      };
    }
    this.statePersistenceService.saveWorkspacesState(stateToSave);
  }
}

export type PendingAccessRequest = {
  [k in ResourceKind]: Record<string, string>;
};
