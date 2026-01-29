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

/**
 * GroupState represents the current state of a rollout group.
 * - `unstarted`: Group hasn't started updating yet.
 * - `canary`: Canary agents are being updated first to make sure the rollout is safe to proceed with.
 * - `active`: Group is currently updating its agents.
 * - `done`: All agents in the group have been updated.
 * - `rolledback`: Group was rolled back to the previous version.
 */
export type GroupState =
  | 'unstarted'
  | 'canary'
  | 'active'
  | 'done'
  | 'rolledback';

export type RolloutStrategy = 'time-based' | 'halt-on-error';

export type GroupAction = 'start' | 'done' | 'rollback';

/**
 * ToolsAutoUpdateInfo contains information about the tools autoupdates configuration.
 */
export interface ToolsAutoUpdateInfo {
  /** Mode is the tools autoupdate mode, either enabled or disabled. */
  mode: 'enabled' | 'disabled';
  /** TargetVersion is the target version. */
  targetVersion: string;
}

/**
 * ClusterMaintenanceInfo contains information about the cluster maintenance configuration.
 * This is for Cloud only.
 */
export interface ClusterMaintenanceInfo {
  /** ControlPlaneVersion is the current version of the control plane. */
  controlPlaneVersion: string;
  /** MaintenanceWeekdays is the list of days when maintenance can occur. */
  maintenanceWeekdays?: string[];
  /** MaintenanceStartHour is the maintenance window start hour (UTC time). */
  maintenanceStartHour: number;
}

/**
 * RolloutInfo contains information about the agent rollout configuration.
 */
export interface RolloutInfo {
  /** StartVersion is the starting version for the rollout. */
  startVersion: string;
  /** TargetVersion is the version to update to in the rollout. */
  targetVersion: string;
  /** Strategy is the rollout strategy, either halt-on-error or time-based. */
  strategy: RolloutStrategy;
  /** Schedule is the rollout schedule. */
  schedule: string;
  /** State is the current state of the rollout. */
  state: string;
  /** Mode is the autoupdate mode option. */
  mode: string;
}

/**
 * RolloutGroupInfo contains information about a rollout group.
 */
export interface RolloutGroupInfo {
  /** Name is the group name. */
  name: string;
  /** Position is the position of this group in the rollout order, this is only applicable for halt-on-error strategy. */
  position?: number;
  /** State is this group's current state in the rollout. */
  state: GroupState;
  /** InitialCount is the number of agents in this group when the rollout started. */
  initialCount: number;
  /** PresentCount is the number of agents in this group currently connected. */
  presentCount: number;
  /** UpToDateCount is the number of agents in this group running the target version. */
  upToDateCount: number;
  /** StateReason is the optional state reason text. */
  stateReason?: string;
  /** StartTime is the time when the group rollout started. */
  startTime?: string;
  /** LastUpdateTime is the time of the last state update for this group. */
  lastUpdateTime?: string;
  /** AgentVersionCounts is a map representing how many agents in this group are on each version. */
  agentVersionCounts?: Record<string, number>;
  /** CanaryCount is the number of canary agents that need to be updated before the whole group is updated. */
  canaryCount?: number;
  /** CanarySuccessCount is the number of canary agents that have been successfully updated. */
  canarySuccessCount?: number;
  /** IsCatchAll indicates whether this group is the catch-all group for orphaned agents. */
  isCatchAll?: boolean;
}

/**
 * ManagedUpdatesDetails is the response to the request for managed updates details.
 */
export interface ManagedUpdatesDetails {
  /** Rollout contains information about the agent rollout configuration. */
  rollout?: RolloutInfo;
  /** Tools contains information about the tools autoupdates configuration. */
  tools?: ToolsAutoUpdateInfo;
  /** ClusterMaintenance contains information about the cluster maintenance configuration. This is for Cloud only. */
  clusterMaintenance?: ClusterMaintenanceInfo;
  /** Groups contains information about rollout groups. */
  groups?: RolloutGroupInfo[];
  /**
   * OrphanedAgentVersionCounts is a map representing how many orphaned agents are in each version.
   * An orphaned agent is an agent belonging to a group that is not defined in the rollout.
   */
  orphanedAgentVersionCounts?: Record<string, number>;
}

/**
 * GroupActionResponse is the response for a group action like start update, mark done, and rollback.
 */
export interface GroupActionResponse {
  /** Group contains the updated group information. */
  group?: RolloutGroupInfo;
}
