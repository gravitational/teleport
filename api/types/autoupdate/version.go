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

// NewAutoUpdateVersion creates a new auto update version resource.
func NewAutoUpdateVersion(spec *autoupdate.AutoUpdateVersionSpec) (*autoupdate.AutoUpdateVersion, error) {
	version := &autoupdate.AutoUpdateVersion{
		Kind:    types.KindAutoUpdateVersion,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: types.MetaNameAutoUpdateVersion,
		},
		Spec: spec,
	}
	if err := ValidateAutoUpdateVersion(version); err != nil {
		return nil, trace.Wrap(err)
	}

	return version, nil
}

// ValidateAutoUpdateVersion checks that required parameters are set
// for the specified AutoUpdateVersion.
func ValidateAutoUpdateVersion(v *autoupdate.AutoUpdateVersion) error {
	if v == nil {
		return trace.BadParameter("AutoUpdateVersion is nil")
	}
	if v.Metadata == nil {
		return trace.BadParameter("Metadata is nil")
	}
	if v.Metadata.Name != types.MetaNameAutoUpdateVersion {
		return trace.BadParameter("Name is not valid")
	}
	if v.Spec == nil {
		return trace.BadParameter("Spec is nil")
	}

	if v.Spec.Tools != nil {
		if err := checkVersion(v.Spec.Tools.TargetVersion); err != nil {
			return trace.Wrap(err, "validating spec.tools.target_version")
		}
	}
	if v.Spec.Agents != nil {
		if err := checkVersion(v.Spec.Agents.StartVersion); err != nil {
			return trace.Wrap(err, "validating spec.agents.start_version")
		}
		if err := checkVersion(v.Spec.Agents.TargetVersion); err != nil {
			return trace.Wrap(err, "validating spec.agents.target_version")
		}
		if err := checkAgentsMode(v.Spec.Agents.Mode); err != nil {
			return trace.Wrap(err, "validating spec.agents.mode")
		}
		if err := checkScheduleName(v.Spec.Agents.Schedule); err != nil {
			return trace.Wrap(err, "validating spec.agents.schedule")
		}
	}

	return nil
}
