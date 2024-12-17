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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

// NewAutoUpdateAgentRollout creates a new auto update version resource.
func NewAutoUpdateAgentRollout(spec *autoupdate.AutoUpdateAgentRolloutSpec) (*autoupdate.AutoUpdateAgentRollout, error) {
	rollout := &autoupdate.AutoUpdateAgentRollout{
		Kind:    types.KindAutoUpdateAgentRollout,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: types.MetaNameAutoUpdateAgentRollout,
		},
		Spec: spec,
	}
	if err := ValidateAutoUpdateAgentRollout(rollout); err != nil {
		return nil, trace.Wrap(err)
	}

	return rollout, nil
}

// ValidateAutoUpdateAgentRollout checks that required parameters are set
// for the specified AutoUpdateAgentRollout.
func ValidateAutoUpdateAgentRollout(v *autoupdate.AutoUpdateAgentRollout) error {
	if v == nil {
		return trace.BadParameter("AutoUpdateAgentRollout is nil")
	}
	if v.Metadata == nil {
		return trace.BadParameter("Metadata is nil")
	}
	if v.Metadata.Name != types.MetaNameAutoUpdateAgentRollout {
		return trace.BadParameter("Name is not valid")
	}
	if v.Spec == nil {
		return trace.BadParameter("Spec is nil")
	}
	if err := checkVersion(v.Spec.StartVersion); err != nil {
		return trace.Wrap(err, "validating spec.start_version")
	}
	if err := checkVersion(v.Spec.TargetVersion); err != nil {
		return trace.Wrap(err, "validating spec.target_version")
	}
	if err := checkAgentsMode(v.Spec.AutoupdateMode); err != nil {
		return trace.Wrap(err, "validating spec.autoupdate_mode")
	}
	if err := checkScheduleName(v.Spec.Schedule); err != nil {
		return trace.Wrap(err, "validating spec.schedule")
	}
	if err := checkAgentsStrategy(v.Spec.Strategy); err != nil {
		return trace.Wrap(err, "validating spec.strategy")
	}

	groups := v.GetStatus().GetGroups()
	seenGroups := make(map[string]int, len(groups))
	for i, group := range groups {
		if group.Name == "" {
			return trace.BadParameter("status.groups[%d].name is empty", i)
		}
		if _, err := types.ParseWeekdays(group.ConfigDays); err != nil {
			return trace.BadParameter("status.groups[%d].config_days is invalid", i)
		}
		if group.ConfigStartHour > 23 || group.ConfigStartHour < 0 {
			return trace.BadParameter("spec.agents.schedules.regular[%d].start_hour must be less than or equal to 23", i)
		}
		if group.ConfigWaitHours < 0 {
			return trace.BadParameter("status.schedules.groups[%d].config_wait_hours cannot be negative", i)
		}
		if v.Spec.Strategy == AgentsStrategyTimeBased && group.ConfigWaitHours != 0 {
			return trace.BadParameter("status.schedules.groups[%d].config_wait_hours must be zero when strategy is %s", i, AgentsStrategyTimeBased)
		}
		if v.Spec.Strategy == AgentsStrategyHaltOnError && i == 0 && group.ConfigWaitHours != 0 {
			return trace.BadParameter("status.schedules.groups[0].config_wait_hours must be zero as it's the first group")
		}
		if conflictingGroup, ok := seenGroups[group.Name]; ok {
			return trace.BadParameter("spec.agents.schedules.regular contains groups with the same name %q at indices %d and %d", group.Name, conflictingGroup, i)
		}
		seenGroups[group.Name] = i
	}

	return nil
}
