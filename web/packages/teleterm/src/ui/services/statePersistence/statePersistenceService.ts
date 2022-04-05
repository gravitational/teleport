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

import { FileStorage } from 'teleterm/types';
import { ConnectionTrackerState } from 'teleterm/ui/services/connectionTracker';
import { WorkspacesState } from 'teleterm/ui/services/workspacesService';
import { debounce } from 'lodash';

interface StatePersistenceState {
  connectionTracker: ConnectionTrackerState;
  workspacesState?: WorkspacesState;
}

export class StatePersistenceService {
  state: StatePersistenceState = {
    connectionTracker: {
      connections: [],
    },
    workspacesState: {
      workspaces: {},
    },
  };
  private readonly putIntoFileStorage: (path: string, json: any) => void;

  constructor(private _fileStorage: FileStorage) {
    const restored = this._fileStorage.get<StatePersistenceState>('state');
    if (restored) {
      this.state = restored;
    }
    // TODO(gzdunek) increase debounce value, run additional save before closing the app
    this.putIntoFileStorage = debounce(this._fileStorage.put, 500);
  }

  saveConnectionTrackerState(navigatorState: ConnectionTrackerState): void {
    this.state.connectionTracker = navigatorState;
    this.putIntoFileStorage('state', this.state);
  }

  getConnectionTrackerState(): ConnectionTrackerState {
    return this.state.connectionTracker;
  }

  saveWorkspaces(workspacesState: WorkspacesState): void {
    this.state.workspacesState.rootClusterUri = workspacesState.rootClusterUri;
    for (let w in workspacesState.workspaces) {
      if (workspacesState.workspaces[w]) {
        this.state.workspacesState.workspaces[w] = {
          location: workspacesState.workspaces[w].location,
          localClusterUri: workspacesState.workspaces[w].localClusterUri,
          documents: workspacesState.workspaces[w].documents,
        };
      }
    }
    this.putIntoFileStorage('state', this.state);
  }

  getWorkspaces(): WorkspacesState {
    return this.state.workspacesState;
  }
}
