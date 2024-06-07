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

export type WorkspacesPersistedState = Omit<
  WorkspacesState,
  'workspaces' | 'isInitialized'
> & {
  workspaces: Record<string, Omit<Workspace, 'accessRequests'>>;
};

export interface StatePersistenceState {
  connectionTracker: ConnectionTrackerState;
  workspacesState: WorkspacesPersistedState;
  shareFeedback: ShareFeedbackState;
  usageReporting: UsageReportingState;
  vnet: { autoStart: boolean };
}

// Before adding new methods to this service, consider using usePersistedState instead.
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

  getState(): StatePersistenceState {
    // Some legacy callsites expected StatePersistenceService to manage the default state for them,
    // but with the move towards usePersistedState, we should put the default state close to where
    // it's going to be used. Hence the use of Partial<StatePersistenceState> here.
    const defaultState: Partial<StatePersistenceState> = {
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

  putState(state: StatePersistenceState): void {
    this._fileStorage.put('state', state);
  }
}
