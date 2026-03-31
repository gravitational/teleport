/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package scopes

import (
	"os"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	apiutils "github.com/gravitational/teleport/api/utils"
)

const (
	// featureVarName is the name of the unstable scopes feature flag.
	featureVarName = "TELEPORT_UNSTABLE_SCOPES"
	// mwiFeatureVarName is the name of the unstable MWI scopes feature flag.
	// Both this and TELEPORT_UNSTABLE_SCOPES must be enabled to use MWI scopes.
	mwiFeatureVarName = "TELEPORT_UNSTABLE_SCOPES_MWI"
)

// FeatureEnabled checks if the scopes feature is enabled.
func FeatureEnabled() bool {
	enabled, err := apiutils.ParseBool(os.Getenv(featureVarName))
	return enabled && err == nil
}

// AssertFeatureEnabled checks if the scopes feature is enabled, and returns a helpful
// error message if it is not.
func AssertFeatureEnabled() error {
	if !FeatureEnabled() {
		return trace.Errorf("scoping features are not enabled, set " + featureVarName + "=yes to enable scoping features (caution: not ready for production use)")
	}

	return nil
}

// ScopesStatusToString returns a user friendly status message based on [proto.ScopesStatus].
func ScopesStatusToString(s proto.ScopesStatus) string {
	switch s {
	case proto.ScopesStatus_SCOPES_STATUS_ENABLED:
		return "enabled"
	case proto.ScopesStatus_SCOPES_STATUS_DISABLED:
		return "disabled"
	default:
		return "unknown"
	}
}

// MWIFeatureEnabled checks if the MWI scopes feature is enabled. It returns
// true only if both MWI Scopes and general Scopes features are enabled.
func MWIFeatureEnabled() bool {
	if !FeatureEnabled() {
		return false
	}

	enabled, err := apiutils.ParseBool(os.Getenv(mwiFeatureVarName))
	return enabled && err == nil
}

// AssertMWIFeatureEnabled checks if the MWI scopes feature is enabled, and
// returns a helpful error message if it is not.
func AssertMWIFeatureEnabled() error {
	if !MWIFeatureEnabled() {
		return trace.Errorf("MWI scoping features are not enabled, set " + mwiFeatureVarName + "=yes and " + featureVarName + "=yes to enable MWI scoping features (caution: not ready for production use)")
	}
	return nil
}
