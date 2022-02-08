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

interface WorkspaceState {
  connectionTracker: ConnectionTrackerState;
  layout: {
    navigatorWidth?: number;
  };
}

export class WorkspaceService {
  private _state: WorkspaceState = {
    connectionTracker: {
      connections: [],
    },
    layout: {
      navigatorWidth: 0,
    },
  };

  constructor(private _fileStorage: FileStorage) {
    const restored = this._fileStorage.get<WorkspaceState>('workspace');
    if (restored) {
      this._state = restored;
    }
  }

  saveConnectionTrackerState(navigatorState: ConnectionTrackerState): void {
    this._state.connectionTracker = navigatorState;
    this._fileStorage.put('workspace', this._state);
  }

  getConnectionTrackerState(): ConnectionTrackerState {
    return this._state.connectionTracker;
  }

  saveNavigatorWidth(width: number): void {
    this._state.layout.navigatorWidth = width;
    this._fileStorage.put('workspace', this._state);
  }

  getNavigatorWidth(): number {
    return this._state.layout.navigatorWidth;
  }
}
