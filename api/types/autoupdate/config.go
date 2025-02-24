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

		if err := checkAgentSchedules(c); err != nil {
			return trace.Wrap(err, "validating spec.agents.schedules")
		}
	}

	return nil
}

func checkAgentSchedules(c *autoupdate.AutoUpdateConfig) error {
	// Validate groups
	groups := c.Spec.Agents.GetSchedules().GetRegular()
	seenGroups := make(map[string]int, len(groups))
	for i, group := range groups {
		if group.Name == "" {
			return trace.BadParameter("spec.agents.schedules.regular[%d].name should not be empty", i)
		}
		if _, err := types.ParseWeekdays(group.Days); err != nil {
			return trace.Wrap(err, "validating spec.agents.schedules.regular[%d].days", i)
		}
		if group.WaitHours < 0 {
			return trace.BadParameter("spec.agents.schedules.regular[%d].wait_hours cannot be negative", i)
		}
		if group.StartHour > 23 || group.StartHour < 0 {
			return trace.BadParameter("spec.agents.schedules.regular[%d].start_hour must be between 0 and 23", i)
		}
		if c.Spec.Agents.Strategy == AgentsStrategyTimeBased && group.WaitHours != 0 {
			return trace.BadParameter("spec.agents.schedules.regular[%d].wait_hours must be zero when strategy is %s", i, AgentsStrategyTimeBased)
		}
		if c.Spec.Agents.Strategy == AgentsStrategyHaltOnError && i == 0 && group.WaitHours != 0 {
			return trace.BadParameter("spec.agents.schedules.regular[0].wait_hours must be zero as it's the first group")
		}
		if conflictingGroup, ok := seenGroups[group.Name]; ok {
			return trace.BadParameter("spec.agents.schedules.regular contains groups with the same name %q at indices %d and %d", group.Name, conflictingGroup, i)
		}
		seenGroups[group.Name] = i
	}
	return nil
}
