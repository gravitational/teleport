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

import cfg from 'teleport/config';
import {
  GroupState,
  ManagedUpdatesDetails,
  RolloutGroupInfo,
} from 'teleport/services/managedUpdates';

/**
 * getProgress returns the rollout progress percentage for a given group.
 */
export function getProgress(group: RolloutGroupInfo): number {
  if (group.presentCount === 0) return 0;
  return Math.round((group.upToDateCount / group.presentCount) * 100);
}

/**
 * getStateOrder returns a numeric order for sorting group states, this is to make sure
 * the status column is sorted by position in the rollout process instead of alphabetically.
 */
export function getStateOrder(state: GroupState): number {
  const order = {
    rolledback: 1,
    unstarted: 2,
    canary: 3,
    active: 4,
    done: 5,
  };
  return order[state] ?? 0;
}

/**
 * getOrphanedCount returns the total count of orphaned agents across all versions.
 */
export function getOrphanedCount(
  orphanedAgentVersionCounts?: Record<string, number>
): number {
  if (!orphanedAgentVersionCounts) return 0;
  return Object.values(orphanedAgentVersionCounts).reduce((a, b) => a + b, 0);
}

/**
 * getReadableStateReason maps backend state reason values to friendly strings.
 * These should be kept in sync with the backend definitions in lib/autoupdate/rollout
 */
export function getReadableStateReason(reason: string): string {
  if (!reason) return 'None';

  const reasonMap: Record<string, string> = {
    can_start: 'Ready to start',
    cannot_start: 'Cannot start',
    previous_groups_not_done: 'Waiting for previous group(s) to complete',
    update_complete: 'Update complete',
    update_in_progress: 'Update in progress',
    canaries_are_alive: 'Canaries are alive',
    waiting_for_canaries: 'Waiting for canaries',
    in_window: 'In maintenance window',
    outside_window: 'Outside maintenance window',
    created: 'Created',
    reconciler_error: 'Reconciler error',
    rollout_changed_during_window: 'Rollout changed during window',
    manual_trigger: 'Manually triggered',
    manual_forced_done: 'Manually marked as done',
    manual_rollback: 'Manually rolled back',
  };
  return reasonMap[reason] || reason;
}

export function isModeEnabled(mode?: string): boolean {
  const normalized = mode?.toLowerCase();
  return !!normalized && normalized !== 'disabled' && normalized !== '';
}

/**
 * checkIsConfigured returns true if managed updates are configured.
 */
export function checkIsConfigured(data: ManagedUpdatesDetails): boolean {
  if (!data) return false;
  return isModeEnabled(data.rollout?.mode) || isModeEnabled(data.tools?.mode);
}

/**
 * getMissingPermissions returns an array of permission names the user is missing.
 */
export function getMissingPermissions({
  canReadConfig,
  canReadVersion,
  canReadRollout,
}: {
  canReadConfig: boolean;
  canReadVersion: boolean;
  canReadRollout: boolean;
}): string[] {
  const missing: string[] = [];
  if (!canReadConfig) missing.push('autoupdate_config.read');
  if (!canReadVersion) missing.push('autoupdate_version.read');
  if (!canReadRollout) missing.push('autoupdate_agent_rollout.read');
  return missing;
}

/**
 * getInstanceInventoryUrlFilteredByGroup returns a URL to the instance inventory
 * filtered by the given group name.
 */
export function getInstanceInventoryUrlFilteredByGroup(
  groupName: string
): string {
  const query = encodeURIComponent(`spec.updater_group == "${groupName}"`);
  return `${cfg.routes.instances}?query=${query}&is_advanced=true`;
}
