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

	// agentPinVarName is the name of the unstable agent scope pin feature flag.
	agentPinVarName = "TELEPORT_UNSTABLE_AGENT_SCOPE_PIN"
)

// Features describes which scopes-related functionality is enabled.
type Features struct {
	// Enabled indicates whether the base scopes feature is enabled.
	Enabled bool

	// AgentPinEnabled checks if the agent scope pin feature is enabled.
	AgentPinEnabled bool
}

// AssertEnabled returns an error if the base scopes feature is disabled.
func (f Features) AssertEnabled() error {
	if !f.Enabled {
		return trace.Errorf("scoping features are not enabled, set " + featureVarName + "=yes to enable scoping features (caution: not ready for production use)")
	}

	return nil
}

// FeaturesFromEnv builds Features from scopes-related environment variables.
func FeaturesFromEnv() Features {
	var f Features
	enabled, err := apiutils.ParseBool(os.Getenv(featureVarName))
	f.Enabled = enabled && err == nil
	agentPinEnabled, err := apiutils.ParseBool(os.Getenv(agentPinVarName))
	f.AgentPinEnabled = agentPinEnabled && err == nil
	return f
}

// AssertFeatureEnabled checks if the scopes feature is enabled, and returns a helpful
// error message if it is not.
// Deprecated: inject scopes.Features instead.
func AssertFeatureEnabled() error {
	if !FeaturesFromEnv().Enabled {
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
