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

package servicecfg

import (
	"io"

	"github.com/gravitational/trace"
)

// PAMConfig holds the configuration used by Teleport when creating a PAM context
// and executing PAM transactions.
type PAMConfig struct {
	// Enabled controls if PAM checks will occur or not.
	Enabled bool

	// ServiceName is the name of the policy to apply typically in /etc/pam.d/
	ServiceName string

	// Login is the *nix login that that is being used.
	Login string `json:"login"`

	// Env is a list of extra environment variables to pass to the PAM modules.
	Env map[string]string

	// Stdin is the input stream which the conversation function will use to
	// obtain data from the user.
	Stdin io.Reader

	// Stdout is the output stream which the conversation function will use to
	// show data to the user.
	Stdout io.Writer

	// Stderr is the output stream which the conversation function will use to
	// report errors to the user.
	Stderr io.Writer

	// UsePAMAuth specifies whether to trigger the "auth" PAM modules from the
	// policy.
	UsePAMAuth bool

	// Environment represents environment variables to pass to PAM.
	// These may contain role-style interpolation syntax.
	Environment map[string]string
}

// CheckDefaults makes sure the PAMConfig structure has minimum required values.
func (c *PAMConfig) CheckDefaults() error {
	if c.ServiceName == "" {
		return trace.BadParameter("required parameter ServiceName missing")
	}
	if c.Login == "" {
		return trace.BadParameter("login parameter required")
	}
	if c.Stdin == nil {
		return trace.BadParameter("required parameter Stdin missing")
	}
	if c.Stdout == nil {
		return trace.BadParameter("required parameter Stdout missing")
	}
	if c.Stderr == nil {
		return trace.BadParameter("required parameter Stderr missing")
	}

	return nil
}
