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
	case AgentsScheduleImmediate:
		return nil
	case AgentsScheduleRegular:
		return trace.BadParameter("regular schedule is not implemented yet")
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
