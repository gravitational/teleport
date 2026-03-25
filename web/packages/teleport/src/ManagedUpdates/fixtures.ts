/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import {
  ClusterMaintenanceInfo,
  ManagedUpdatesDetails,
  RolloutGroupInfo,
  RolloutInfo,
  ToolsAutoUpdateInfo,
} from 'teleport/services/managedUpdates';

const now = new Date();
const oneWeekAgo = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000);
const oneDayAgo = new Date(now.getTime() - 1 * 24 * 60 * 60 * 1000);
const fiveHoursAgo = new Date(now.getTime() - 5 * 60 * 60 * 1000);
const threeHoursAgo = new Date(now.getTime() - 3 * 60 * 60 * 1000);

export const mockTools: ToolsAutoUpdateInfo = {
  mode: 'enabled',
  targetVersion: 'v18.2.5',
};

export const mockToolsDisabled: ToolsAutoUpdateInfo = {
  mode: 'disabled',
  targetVersion: '',
};

export const mockClusterMaintenance: ClusterMaintenanceInfo = {
  controlPlaneVersion: 'v18.2.5',
  maintenanceWeekdays: ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday'],
  maintenanceStartHour: 3,
};

export const mockRollout: RolloutInfo = {
  startVersion: 'v18.2.1',
  targetVersion: 'v18.2.5',
  strategy: 'time-based',
  schedule: 'regular',
  state: 'active',
  mode: 'enabled',
};

export const mockRolloutHaltOnError: RolloutInfo = {
  startVersion: 'v18.2.1',
  targetVersion: 'v18.2.5',
  strategy: 'halt-on-error',
  schedule: 'regular',
  state: 'active',
  mode: 'enabled',
};

export const mockRolloutImmediate: RolloutInfo = {
  startVersion: 'v18.2.1',
  targetVersion: 'v18.2.5',
  strategy: 'time-based',
  schedule: 'immediate',
  state: 'active',
  mode: 'enabled',
};

export const mockGroups: RolloutGroupInfo[] = [
  {
    name: 'dev',
    position: 1,
    state: 'done',
    initialCount: 120,
    presentCount: 120,
    upToDateCount: 120,
    stateReason: 'update_complete',
    startTime: oneWeekAgo.toISOString(),
    lastUpdateTime: oneWeekAgo.toISOString(),
    agentVersionCounts: {
      'v18.2.5': 120,
    },
  },
  {
    name: 'staging',
    position: 2,
    state: 'canary',
    initialCount: 156,
    presentCount: 156,
    upToDateCount: 41,
    stateReason: 'waiting_for_canaries',
    startTime: oneWeekAgo.toISOString(),
    lastUpdateTime: oneDayAgo.toISOString(),
    agentVersionCounts: {
      'v18.2.1': 115,
      'v18.2.5': 41,
    },
    canaryCount: 5,
    canarySuccessCount: 2,
  },
  {
    name: 'prod',
    position: 3,
    state: 'active',
    initialCount: 120,
    presentCount: 120,
    upToDateCount: 2,
    stateReason: 'update_in_progress',
    startTime: oneWeekAgo.toISOString(),
    lastUpdateTime: fiveHoursAgo.toISOString(),
    agentVersionCounts: {
      'v18.2.1': 118,
      'v18.2.5': 2,
    },
  },
  {
    name: 'backup',
    position: 4,
    state: 'rolledback',
    initialCount: 30,
    presentCount: 30,
    upToDateCount: 0,
    stateReason: 'manual_rollback',
    startTime: oneWeekAgo.toISOString(),
    lastUpdateTime: threeHoursAgo.toISOString(),
    agentVersionCounts: {
      'v18.2.1': 30,
    },
  },
  {
    name: 'govcloud',
    position: 5,
    state: 'unstarted',
    initialCount: 12,
    presentCount: 12,
    upToDateCount: 0,
    stateReason: 'previous_groups_not_done',
    agentVersionCounts: {
      'v18.2.1': 12,
    },
    isCatchAll: true,
  },
];

export const mockGroupsWithOrphaned: RolloutGroupInfo[] = [
  {
    name: 'dev',
    position: 1,
    state: 'done',
    initialCount: 120,
    presentCount: 120,
    upToDateCount: 120,
    stateReason: 'update_complete',
    startTime: oneWeekAgo.toISOString(),
    lastUpdateTime: oneWeekAgo.toISOString(),
    agentVersionCounts: {
      'v18.2.5': 120,
    },
  },
  {
    name: 'staging',
    position: 2,
    state: 'active',
    initialCount: 156,
    presentCount: 156,
    upToDateCount: 100,
    stateReason: 'update_in_progress',
    startTime: oneWeekAgo.toISOString(),
    lastUpdateTime: oneDayAgo.toISOString(),
    agentVersionCounts: {
      'v18.2.1': 56,
      'v18.2.5': 100,
    },
  },
  {
    name: 'prod',
    position: 3,
    state: 'unstarted',
    initialCount: 80,
    presentCount: 80,
    upToDateCount: 0,
    stateReason: 'previous_groups_not_done',
    agentVersionCounts: {
      'v18.2.1': 80,
    },
    isCatchAll: true,
  },
];

export const mockOrphanedAgentVersionCounts: Record<string, number> = {
  'v18.2.1': 15,
  'v18.2.3': 3,
  'v18.2.5': 7,
};

export const mockManagedUpdatesTimeBased: ManagedUpdatesDetails = {
  tools: mockTools,
  rollout: mockRollout,
  groups: mockGroups,
};

export const mockManagedUpdatesHaltOnError: ManagedUpdatesDetails = {
  tools: mockTools,
  rollout: mockRolloutHaltOnError,
  groups: mockGroups,
};

export const mockManagedUpdatesWithOrphaned: ManagedUpdatesDetails = {
  tools: mockTools,
  rollout: mockRolloutHaltOnError,
  groups: mockGroupsWithOrphaned,
  orphanedAgentVersionCounts: mockOrphanedAgentVersionCounts,
};

export const mockManagedUpdatesImmediate: ManagedUpdatesDetails = {
  tools: mockTools,
  rollout: mockRolloutImmediate,
  groups: mockGroups,
};

export const mockGroupsWithAgentsDropped: RolloutGroupInfo[] = [
  {
    name: 'dev',
    position: 1,
    state: 'done',
    initialCount: 120,
    presentCount: 120,
    upToDateCount: 120,
    stateReason: 'update_complete',
    startTime: oneWeekAgo.toISOString(),
    lastUpdateTime: oneWeekAgo.toISOString(),
    agentVersionCounts: {
      'v18.2.5': 120,
    },
  },
  {
    name: 'staging',
    position: 2,
    state: 'active',
    initialCount: 100,
    presentCount: 85,
    upToDateCount: 80,
    stateReason: 'update_in_progress',
    startTime: oneWeekAgo.toISOString(),
    lastUpdateTime: fiveHoursAgo.toISOString(),
    agentVersionCounts: {
      'v18.2.1': 5,
      'v18.2.5': 80,
    },
  },
  {
    name: 'prod',
    position: 3,
    state: 'unstarted',
    initialCount: 80,
    presentCount: 80,
    upToDateCount: 0,
    stateReason: 'previous_groups_not_done',
    agentVersionCounts: {
      'v18.2.1': 80,
    },
    isCatchAll: true,
  },
];

export const mockManagedUpdatesAgentsDropped: ManagedUpdatesDetails = {
  tools: mockTools,
  rollout: mockRolloutHaltOnError,
  groups: mockGroupsWithAgentsDropped,
};

export const mockManagedUpdatesNotConfigured: ManagedUpdatesDetails = {
  tools: mockToolsDisabled,
  // No rollout means not configured
};

export const mockManagedUpdatesNotConfiguredCloud: ManagedUpdatesDetails = {
  tools: mockToolsDisabled,
  clusterMaintenance: mockClusterMaintenance,
};

export const mockManagedUpdatesDetailsEmpty: ManagedUpdatesDetails = {
  tools: {
    mode: 'disabled',
    targetVersion: '',
  },
};

export const mockManagedUpdatesDetailsNoGroups: ManagedUpdatesDetails = {
  tools: mockTools,
  rollout: {
    ...mockRollout,
    state: 'unstarted',
  },
  groups: [],
};

export const mockSelectedGroup: RolloutGroupInfo = {
  name: 'prod',
  position: 3,
  state: 'active',
  initialCount: 120,
  presentCount: 112,
  upToDateCount: 2,
  stateReason: 'update_in_progress',
  startTime: oneWeekAgo.toISOString(),
  lastUpdateTime: fiveHoursAgo.toISOString(),
  agentVersionCounts: {
    'v18.2.1': 2,
    'v18.2.3': 0,
    'v18.2.5': 118,
  },
  isCatchAll: true,
};

export const mockManagedUpdatesTimeBasedCloud: ManagedUpdatesDetails = {
  tools: mockTools,
  rollout: mockRollout,
  groups: mockGroups,
  clusterMaintenance: mockClusterMaintenance,
};

export const mockManagedUpdatesHaltOnErrorCloud: ManagedUpdatesDetails = {
  tools: mockTools,
  rollout: mockRolloutHaltOnError,
  groups: mockGroups,
  clusterMaintenance: mockClusterMaintenance,
};
