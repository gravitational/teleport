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

const (
	// ToolsUpdateModeEnabled enables client tools automatic updates.
	ToolsUpdateModeEnabled = "enabled"
	// ToolsUpdateModeDisabled disables client tools automatic updates.
	ToolsUpdateModeDisabled = "disabled"
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
		if c.Spec.Tools.Mode != ToolsUpdateModeDisabled && c.Spec.Tools.Mode != ToolsUpdateModeEnabled {
			return trace.BadParameter("ToolsMode is not valid")
		}
	}

	return nil
}
