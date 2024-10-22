package autoupdate

const (
	// ToolsUpdateModeEnabled enables client tools automatic updates.
	ToolsUpdateModeEnabled = "enabled"
	// AgentsUpdateModeEnabled enabled agent automatic updates.
	AgentsUpdateModeEnabled
	// ToolsUpdateModeDisabled disables client tools automatic updates.
	ToolsUpdateModeDisabled = "disabled"
	// AgentsUpdateModeDisabled disables agent automatic updates.
	AgentsUpdateModeDisabled
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
