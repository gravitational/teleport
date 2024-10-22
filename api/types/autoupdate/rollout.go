/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package autoupdate

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

// NewAutoUpdateAgentRollout creates a new auto update version resource.
func NewAutoUpdateAgentRollout(spec *autoupdate.AutoUpdateAgentRolloutSpec) (*autoupdate.AutoUpdateAgentRollout, error) {
	version := &autoupdate.AutoUpdateAgentRollout{
		Kind:    types.KindAutoUpdateAgentRollout,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: types.MetaNameAutoUpdateAgentRollout,
		},
		Spec: spec,
	}
	if err := ValidateAutoUpdateAgentRollout(version); err != nil {
		return nil, trace.Wrap(err)
	}

	return version, nil
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
