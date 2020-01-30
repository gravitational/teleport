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
	"encoding/json"
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

	// LoginContext is additional metadata about the user that Teleport stores in
	// the PAM_RUSER field. It can be extracted by PAM modules like
	// pam_script.so to configure the users environment.
	LoginContext *LoginContextV1

	// Stdin is the input stream which the conversation function will use to
	// obtain data from the user.
	Stdin io.Reader

	// Stdout is the output stream which the conversation function will use to
	// show data to the user.
	Stdout io.Writer

	// Stderr is the output stream which the conversation function will use to
	// report errors to the user.
	Stderr io.Writer
}

// CheckDefaults makes sure the Config structure has minimum required values.
func (c *Config) CheckDefaults() error {
	if c.ServiceName == "" {
		return trace.BadParameter("required parameter ServiceName missing")
	}
	if c.LoginContext == nil {
		return trace.BadParameter("login context required")
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

// LoginContextV1 is passed to PAM modules in the PAM_RUSER field.
type LoginContextV1 struct {
	// Version is the version of this struct.
	Version int `json:"version"`

	// Username is the Teleport user (identity) that is attempting to login.
	Username string `json:"username"`

	// Login is the *nix login that that is being used.
	Login string `json:"login"`

	// Roles is a list of roles assigned to the user.
	Roles []string `json:"roles"`
}

// Marshal marshals the login context into a format that can be passed to
// PAM modules.
func (c *LoginContextV1) Marshal() (string, error) {
	c.Version = 1

	buf, err := json.Marshal(c)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return string(buf), nil
}
