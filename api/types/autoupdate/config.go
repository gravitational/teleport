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
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

// NewAutoUpdateConfig creates a new auto update configuration resource.
func NewAutoUpdateConfig(spec *autoupdate.AutoUpdateConfigSpec) (*autoupdate.AutoUpdateConfig, error) {
	config := &autoupdate.AutoUpdateConfig{
		Kind:    types.KindAutoUpdateConfig,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: types.MetaNameAutoUpdateConfig,
		},
		Spec: spec,
	}
	if err := ValidateAutoUpdateConfig(config); err != nil {
		return nil, trace.Wrap(err)
	}

	return config, nil
}

// ValidateAutoUpdateConfig checks that required parameters are set
// for the specified AutoUpdateConfig.
func ValidateAutoUpdateConfig(c *autoupdate.AutoUpdateConfig) error {
	if c == nil {
		return trace.BadParameter("AutoUpdateConfig is nil")
	}
	if c.Metadata == nil {
		return trace.BadParameter("Metadata is nil")
	}
	if c.Metadata.Name != types.MetaNameAutoUpdateConfig {
		return trace.BadParameter("Name is not valid")
	}
	if c.Spec == nil {
		return trace.BadParameter("Spec is nil")
	}
	if c.Spec.Tools != nil {
		if err := checkToolsMode(c.Spec.Tools.Mode); err != nil {
			return trace.Wrap(err, "validating spec.tools.mode")
		}
	}
	if c.Spec.Agents != nil {
		if err := checkAgentsMode(c.Spec.Agents.Mode); err != nil {
			return trace.Wrap(err, "validating spec.agents.mode")
		}
		if err := checkAgentsStrategy(c.Spec.Agents.Strategy); err != nil {
			return trace.Wrap(err, "validating spec.agents.strategy")
		}

		windowDuration := c.Spec.Agents.MaintenanceWindowDuration.AsDuration()
		if c.Spec.Agents.Strategy == AgentsStrategyHaltOnError && windowDuration != 0 {
			return trace.BadParameter("spec.agents.maintenance_window_duration must be zero when the strategy is %q", c.Spec.Agents.Strategy)
		}
		if c.Spec.Agents.Strategy == AgentsStrategyTimeBased && windowDuration < 10*time.Minute {
			return trace.BadParameter("spec.agents.maintenance_window_duration must be greater than 10 minutes when the strategy is %q", c.Spec.Agents.Strategy)
		}

		if err := checkAgentSchedules(c.Spec.Agents.Schedules); err != nil {
			return trace.Wrap(err, "validating spec.agents.schedules")
		}

	}

	return nil
}

func checkAgentSchedules(schedules *autoupdate.AgentAutoUpdateSchedules) error {
	// TODO: change this logic when we implement group support.
	// Currently we reject any non-nil schedule
	// When we'll implement schedule support, we'll treat an empty schedule as the default schedule.
	if schedules == nil {
		return nil
	}
	return trace.NotImplemented("agent schedules are not implemented yet")
}
