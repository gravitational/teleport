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
