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

	apiutils "github.com/gravitational/teleport/api/utils"
)

const (
	// featureVarName is the name of the unstable scopes feature flag.
	featureVarName = "TELEPORT_UNSTABLE_SCOPES"
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
