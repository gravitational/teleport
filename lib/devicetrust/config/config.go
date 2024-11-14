/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package config

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
)

// GetEffectiveMode returns the effective device trust mode, considering both
// `dt` and the current modules.
func GetEffectiveMode(dt *types.DeviceTrust) string {
	// OSS doesn't support device trust.
	if modules.GetModules().IsOSSBuild() {
		return constants.DeviceTrustModeOff
	}

	// Enterprise defaults to "optional".
	if dt == nil || dt.Mode == "" {
		return constants.DeviceTrustModeOptional
	}

	return dt.Mode
}

// GetEnforcementMode returns the configured device trust mode, disregarding the
// provenance of the binary if the mode is set.
// Used for device enforcement checks. Guarantees that OSS binaries paired with
// an Enterprise Auth will correctly enforce device trust.
func GetEnforcementMode(dt *types.DeviceTrust) string {
	// If absent use the defaults from GetEffectiveMode.
	if dt == nil || dt.Mode == "" {
		return GetEffectiveMode(dt)
	}
	return dt.Mode
}

// ValidateConfigAgainstModules verifies the device trust configuration against
// the current modules.
// This method exists to provide feedback to users about invalid configurations,
// Teleport itself checks the features where appropriate and reacts accordingly.
func ValidateConfigAgainstModules(dt *types.DeviceTrust) error {
	switch {
	case dt == nil || dt.Mode == "": // OK, always allowed.
		return nil
	case GetEffectiveMode(dt) != dt.Mode: // Mismatch means invalid OSS config.
		return trace.BadParameter("device trust mode %q requires Teleport Enterprise", dt.Mode)
	default:
		return nil
	}
}
