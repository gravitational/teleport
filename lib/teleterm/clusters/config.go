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

package clusters

import (
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

// Config is the cluster service config
type Config struct {
	// Dir is the directory to store cluster profiles
	Dir string
	// Clock is a clock for time-related operations
	Clock clockwork.Clock
	// InsecureSkipVerify is an option to skip TLS cert check
	InsecureSkipVerify bool
	// Logger is a component logger
	Logger *slog.Logger
	// WebauthnLogin allows tests to override the Webauthn Login func.
	// Defaults to wancli.Login.
	WebauthnLogin client.WebauthnLoginFunc
	// AddKeysToAgent is passed to [client.Config].
	AddKeysToAgent string
	// CustomHardwareKeyPrompt is a custom hardware key prompt to use when asking
	// for a hardware key PIN, touch, etc.
	HardwareKeyPromptConstructor func(rootClusterURI uri.ResourceURI) keys.HardwareKeyPrompt
}

// CheckAndSetDefaults checks the configuration for its validity and sets default values if needed
func (c *Config) CheckAndSetDefaults() error {
	if c.Dir == "" {
		return trace.BadParameter("missing working directory")
	}

	if c.HardwareKeyPromptConstructor == nil {
		return trace.BadParameter("missing hardware key prompt constructor")
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, "conn:storage")
	}

	if c.AddKeysToAgent == "" {
		c.AddKeysToAgent = client.AddKeysToAgentAuto
	}

	return nil
}
