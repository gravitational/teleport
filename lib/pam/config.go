/*
Copyright 2018 Gravitational, Inc.

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

// Package pam implements a subset of Pluggable Authentication Modules (PAM).
// The supported subset of the PAM stack is "account" and "session" modules.
package pam

import (
	"io"

	"github.com/gravitational/trace"
)

// Config holds the configuration used by Teleport when creating a PAM context
// and executing PAM transactions.
type Config struct {
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

// CheckDefaults makes sure the Config structure has minimum required values.
func (c *Config) CheckDefaults() error {
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
