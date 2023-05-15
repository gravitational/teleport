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
import {
  Workspace,
  WorkspacesState,
} from 'teleterm/ui/services/workspacesService';

interface ShareFeedbackState {
  hasBeenOpened: boolean;
}

interface UsageReportingState {
  askedForUserJobRole: boolean;
}

export type WorkspacesPersistedState = Omit<WorkspacesState, 'workspaces'> & {
  workspaces: Record<string, Omit<Workspace, 'accessRequests'>>;
};

interface StatePersistenceState {
  connectionTracker: ConnectionTrackerState;
  workspacesState: WorkspacesPersistedState;
  shareFeedback: ShareFeedbackState;
  usageReporting: UsageReportingState;
}

export class StatePersistenceService {
  constructor(private _fileStorage: FileStorage) {}

  saveConnectionTrackerState(connectionTracker: ConnectionTrackerState): void {
    const newState: StatePersistenceState = {
      ...this.getState(),
      connectionTracker,
    };
    this.putState(newState);
  }

  getConnectionTrackerState(): ConnectionTrackerState {
    return this.getState().connectionTracker;
  }

  saveWorkspacesState(workspacesState: WorkspacesPersistedState): void {
    const newState: StatePersistenceState = {
      ...this.getState(),
      workspacesState,
    };
    this.putState(newState);
  }

  getWorkspacesState(): WorkspacesPersistedState {
    return this.getState().workspacesState;
  }

  saveShareFeedbackState(shareFeedback: ShareFeedbackState): void {
    const newState: StatePersistenceState = {
      ...this.getState(),
      shareFeedback,
    };
    this.putState(newState);
  }

  getShareFeedbackState(): ShareFeedbackState {
    return this.getState().shareFeedback;
  }

  saveUsageReportingState(usageReporting: UsageReportingState): void {
    const newState: StatePersistenceState = {
      ...this.getState(),
      usageReporting,
    };
    this.putState(newState);
  }

  getUsageReportingState(): UsageReportingState {
    return this.getState().usageReporting;
  }

  private getState(): StatePersistenceState {
    const defaultState: StatePersistenceState = {
      connectionTracker: {
        connections: [],
      },
      workspacesState: {
        workspaces: {},
      },
      shareFeedback: {
        hasBeenOpened: false,
      },
      usageReporting: {
        askedForUserJobRole: false,
      },
    };
    return {
      ...defaultState,
      ...(this._fileStorage.get('state') as StatePersistenceState),
    };
  }

  private putState(state: StatePersistenceState): void {
    this._fileStorage.put('state', state);
  }
}
