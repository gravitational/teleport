package autoupdate

import (
	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
)

func checkVersion(version string) error {
	if version == "" {
		return trace.BadParameter("version is unset")
	}
	if _, err := semver.NewVersion(version); err != nil {
		return trace.BadParameter("version %q is not a valid semantic version", version)
	}
	return nil
}

func checkAgentsMode(mode string) error {
	switch mode {
	case AgentsUpdateModeEnabled, AgentsUpdateModeDisabled, AgentsUpdateModeSuspended:
		return nil
	default:
		return trace.BadParameter("unsupported agents mode: %q", mode)
	}
}

func checkToolsMode(mode string) error {
	switch mode {
	case ToolsUpdateModeEnabled, ToolsUpdateModeDisabled:
		return nil
	default:
		return trace.BadParameter("unsupported tools mode: %q", mode)
	}
}

func checkScheduleName(schedule string) error {
	switch schedule {
	case AgentsScheduleRegular, AgentsScheduleImmediate:
		return nil
	default:
		return trace.BadParameter("unsupported schedule type: %q", schedule)
	}
}

func checkAgentsStrategy(strategy string) error {
	switch strategy {
	case AgentsStrategyHaltOnError, AgentsStrategyTimeBased:
		return nil
	default:
		return trace.BadParameter("unsupported agents strategy: %q", strategy)
	}
}
