/*
Copyright 2024 Gravitational, Inc.

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

package autoupdate

const (
	// ToolsUpdateModeEnabled enables client tools automatic updates.
	ToolsUpdateModeEnabled = "enabled"
	// ToolsUpdateModeDisabled disables client tools automatic updates.
	ToolsUpdateModeDisabled = "disabled"

	// AgentsUpdateModeEnabled enabled agent automatic updates.
	AgentsUpdateModeEnabled = "enabled"
	// AgentsUpdateModeDisabled disables agent automatic updates.
	AgentsUpdateModeDisabled = "disabled"
	// AgentsUpdateModeSuspended temporarily suspends agent automatic updates.
	AgentsUpdateModeSuspended = "suspended"

	// AgentsScheduleRegular is the regular agent update schedule.
	AgentsScheduleRegular = "regular"
	// AgentsScheduleImmediate is the immediate agent update schedule.
	// Every agent must update immediately if it's not already running the target version.
	// This can be used to recover agents in case of major incident or actively exploited vulnerability.
	AgentsScheduleImmediate = "immediate"

	// AgentsStrategyHaltOnError is the agent update strategy that updates groups sequentially
	// according to their order in the schedule. The previous groups must succeed.
	AgentsStrategyHaltOnError = "halt-on-error"
	// AgentsStrategyTimeBased is the agent update strategy that updates groups solely based on their
	// maintenance window. There is no dependency between groups. Agents won't be instructed to update
	// if the window is over.
	AgentsStrategyTimeBased = "time-based"
)
