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

	return nil
}
