/*
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

package ui

import "time"

// ManagedUpdatesDetails is the response to the request for managed updates details.
type ManagedUpdatesDetails struct {
	// Rollout contains information about the agent rollout configuration.
	Rollout *RolloutInfo `json:"rollout,omitempty"`

	// Tools contains information about the tools autoupdates configuration.
	Tools *ToolsAutoUpdateInfo `json:"tools,omitempty"`

	// ClusterMaintenance contains information about the cluster maintenance configuration. This is for Cloud only.
	ClusterMaintenance *ClusterMaintenanceInfo `json:"clusterMaintenance,omitempty"`

	// Groups contains information about rollout groups.
	Groups []RolloutGroupInfo `json:"groups,omitempty"`

	// OrphanedAgentVersionCounts is a map representing how many orphaned agents are in each version.
	// An orphaned agent is an agent belonging to a group that is not defined in the rollout.
	OrphanedAgentVersionCounts map[string]int `json:"orphanedAgentVersionCounts,omitempty"`
}

// RolloutInfo contains information about the agent rollout configuration.
type RolloutInfo struct {
	// StartVersion is the starting version for the rollout.
	StartVersion string `json:"startVersion,omitempty"`
	// TargetVersion is the version to update to in the rollout.
	TargetVersion string `json:"targetVersion,omitempty"`
	// Strategy is the rollout strategy, either halt-on-error or time-based.
	Strategy string `json:"strategy,omitempty"`
	// Schedule is the rollout schedule.
	Schedule string `json:"schedule,omitempty"`
	// State is the current state of the rollout.
	State string `json:"state,omitempty"`
	// Mode is the autoupdate mode option.
	Mode string `json:"mode,omitempty"`
	// StartTime is when the rollout was created/started.
	StartTime *time.Time `json:"startTime,omitempty"`
}

// ToolsAutoUpdateInfo contains information about the tools autoupdates configuration.
type ToolsAutoUpdateInfo struct {
	// Mode is the tools autoupdate mode, either enabled or disabled.
	Mode string `json:"mode,omitempty"`
	// TargetVersion is the target version.
	TargetVersion string `json:"targetVersion,omitempty"`
}

// ClusterMaintenanceInfo contains information about the cluster maintenance configuration. This is for Cloud only.
type ClusterMaintenanceInfo struct {
	// ControlPlaneVersion is the current version of the control plane.
	ControlPlaneVersion string `json:"controlPlaneVersion,omitempty"`
	// MaintenanceWeekdays is the list of days when maintenance can occur.
	MaintenanceWeekdays []string `json:"maintenanceWeekdays,omitempty"`
	// MaintenanceStartHour is the maintenance window start hour (UTC time).
	MaintenanceStartHour int `json:"maintenanceStartHour"`
}

// RolloutGroupInfo contains information about a rollout group.
type RolloutGroupInfo struct {
	// Name is the group name.
	Name string `json:"name"`
	// Position is the position of this group in the rollout order, this is only applicable for halt-on-error strategy.
	Position int `json:"position,omitempty"`
	// State is this group's current state in the rollout.
	State string `json:"state"`
	// InitialCount is the number of agents in this group when the rollout started.
	InitialCount uint64 `json:"initialCount"`
	// PresentCount is the number of agents in this group currently connected.
	PresentCount uint64 `json:"presentCount"`
	// UpToDateCount is the number of agents in this group running the target version.
	UpToDateCount uint64 `json:"upToDateCount"`
	// StateReason is the optional state reason text.
	StateReason string `json:"stateReason,omitempty"`
	// StartTime is the time when the group rollout started.
	StartTime *time.Time `json:"startTime,omitempty"`
	// LastUpdateTime is the time of the last state update for this group.
	LastUpdateTime *time.Time `json:"lastUpdateTime,omitempty"`
	// AgentVersionCounts is a map representing how many agents in this group are on each version.
	AgentVersionCounts map[string]int `json:"agentVersionCounts,omitempty"`
	// CanaryCount is the number of canary agents that need to be updated before the whole group is updated.
	CanaryCount uint64 `json:"canaryCount,omitempty"`
	// CanarySuccessCount is the number of canary agents that have been successfully updated.
	CanarySuccessCount uint64 `json:"canarySuccessCount,omitempty"`
	// IsCatchAll indicates whether this group is the catch-all group for orphaned agents.
	IsCatchAll bool `json:"isCatchAll,omitempty"`
}

// StartGroupUpdateRequest is the request body for starting a group update.
type StartGroupUpdateRequest struct {
	// Force, if true, skips canary phase and goes directly to active state.
	Force bool `json:"force,omitempty"`
}

// GroupActionResponse is the response for a group action like start update, mark done, and rollback.
type GroupActionResponse struct {
	// Group contains the updated group information.
	Group *RolloutGroupInfo `json:"group,omitempty"`
}
