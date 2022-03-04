import { ImmutableStore } from 'teleterm/ui/services/immutableStore';
import {
  Document,
  DocumentsService,
} from 'teleterm/ui/services/workspacesService/documentsService';
import { useStore } from 'shared/libs/stores';
import { ModalsService } from 'teleterm/ui/services/modals';
import { ClustersService } from 'teleterm/ui/services/clusters';
import { StatePersistenceService } from 'teleterm/ui/services/statePersistence';

export interface WorkspacesState {
  rootClusterUri?: string;
  workspaces: Record<string, Workspace>;
}

export interface Workspace {
  localClusterUri: string;
  documents: Document[];
  location: string;
}

export class WorkspacesService extends ImmutableStore<WorkspacesState> {
  private documentsServicesCache = new Map<string, DocumentsService>();
  state: WorkspacesState = {
    rootClusterUri: undefined,
    workspaces: {},
  };

  constructor(
    private clustersService: ClustersService,
    private modalsService: ModalsService,
    private statePersistenceService: StatePersistenceService
  ) {
    super();
  }

  getRootClusterUri(): string | undefined {
    return this.state.rootClusterUri;
  }

  getWorkspaces(): Record<string, Workspace> {
    return this.state.workspaces;
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
            }),
          clusterUri
        )
      );
    }

    return this.documentsServicesCache.get(clusterUri);
  }

  useState() {
    return useStore(this);
  }

  setState(nextState: (draftState: WorkspacesState) => WorkspacesState | void) {
    super.setState(nextState);
    this.statePersistenceService.saveWorkspaces(this.state);
  }

  setActiveWorkspace(clusterUri: string): Promise<void> {
    const setWorkspace = () => {
      this.setState(draftState => {
        if (!draftState.workspaces[clusterUri]) {
          const persistedWorkspace =
            this.statePersistenceService.getWorkspaces().workspaces[clusterUri];
          draftState.workspaces[clusterUri] = {
            localClusterUri: persistedWorkspace?.localClusterUri,
            location: persistedWorkspace?.location,
            documents: persistedWorkspace?.documents || [],
          };
        }
        draftState.rootClusterUri = clusterUri;
      });
    };

    const isConnected = this.clustersService.findCluster(clusterUri)?.connected;
    return new Promise((resolve, reject) => {
      if (!isConnected) {
        this.modalsService.openClusterConnectDialog(clusterUri, () => {
          setWorkspace();
          resolve();
        });
      } else {
        setWorkspace();
        resolve();
      }

      //TODO: add reject
    });
  }
}
