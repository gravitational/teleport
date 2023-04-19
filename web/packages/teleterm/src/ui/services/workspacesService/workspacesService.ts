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

import { isEqual } from 'lodash';

import { ModalsService } from 'teleterm/ui/services/modals';
import { ClustersService } from 'teleterm/ui/services/clusters';
import { StatePersistenceService } from 'teleterm/ui/services/statePersistence';
import { ImmutableStore } from 'teleterm/ui/services/immutableStore';
import { NotificationsService } from 'teleterm/ui/services/notifications';
import { routing } from 'teleterm/ui/uri';

import { Document, DocumentsService } from './documentsService';

export interface WorkspacesState {
  rootClusterUri?: string;
  workspaces: Record<string, Workspace>;
}

export interface Workspace {
  localClusterUri: string;
  documents: Document[];
  location: string;
  previous?: {
    documents: Document[];
    location: string;
  };
}

export class WorkspacesService extends ImmutableStore<WorkspacesState> {
  private documentsServicesCache = new Map<string, DocumentsService>();
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

  getActiveWorkspace(): Workspace | undefined {
    return this.state.workspaces[this.state.rootClusterUri];
  }

  getRootClusterUri(): string | undefined {
    return this.state.rootClusterUri;
  }

  getWorkspaces(): Record<string, Workspace> {
    return this.state.workspaces;
  }

  getWorkspace(clusterUri): Workspace {
    return this.state.workspaces[clusterUri];
  }

  getActiveWorkspaceDocumentService(): DocumentsService | undefined {
    if (!this.state.rootClusterUri) {
      return;
    }
    return this.getWorkspaceDocumentService(this.state.rootClusterUri);
  }

  getWorkspacesDocumentsServices(): Array<{
    clusterUri: string;
    workspaceDocumentsService: DocumentsService;
  }> {
    return Object.entries(this.state.workspaces).map(([clusterUri]) => ({
      clusterUri,
      workspaceDocumentsService: this.getWorkspaceDocumentService(clusterUri),
    }));
  }

  setWorkspaceLocalClusterUri(
    clusterUri: string,
    localClusterUri: string
  ): void {
    this.setState(draftState => {
      draftState.workspaces[clusterUri].localClusterUri = localClusterUri;
    });
  }

  getWorkspaceDocumentService(
    clusterUri: string
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

  isDocumentActive(documentUri: string): boolean {
    const documentService = this.getActiveWorkspaceDocumentService();
    return documentService && documentService.isActive(documentUri);
  }

  useState() {
    return useStore(this);
  }

  setState(nextState: (draftState: WorkspacesState) => WorkspacesState | void) {
    super.setState(nextState);
    this.persistState();
  }

  setActiveWorkspace(clusterUri: string): Promise<void> {
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

  removeWorkspace(clusterUri: string): void {
    this.setState(draftState => {
      delete draftState.workspaces[clusterUri];
    });
  }

  getConnectedWorkspacesClustersUri(): string[] {
    return Object.keys(this.state.workspaces).filter(
      clusterUri => this.clustersService.findCluster(clusterUri)?.connected
    );
  }

  restorePersistedState(): void {
    const persistedState = this.statePersistenceService.getWorkspacesState();
    const restoredWorkspaces = this.clustersService
      .getClusters()
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

  private reopenPreviousDocuments(clusterUri: string): void {
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
          };
        }
        return d;
      });
      workspace.location = workspace.previous.location;
      workspace.previous = undefined;
    });
  }

  private discardPreviousDocuments(clusterUri: string): void {
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
      !isEqual(
        omitUriAndTitle(previousDocuments),
        omitUriAndTitle(currentDocuments)
      )
    );
  }

  private getWorkspaceDefaultState(localClusterUri: string): Workspace {
    const rootClusterUri = routing.ensureRootClusterUri(localClusterUri);
    const defaultDocument = this.getWorkspaceDocumentService(
      rootClusterUri
    ).createClusterDocument({ clusterUri: localClusterUri });
    return {
      localClusterUri,
      location: defaultDocument.uri,
      documents: [defaultDocument],
    };
  }

  private persistState(): void {
    const stateToSave: WorkspacesState = {
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
