// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

// Target identifies a single function, within a specific package, that
// docs-url-lint should check for a Teleport documentation link.
type Target struct {
	Package  string
	Function string
	// Receiver, if set, restricts matching to a method with this exact
	// receiver type name (e.g. "ProvisionTokenV2" for
	// "func (p *ProvisionTokenV2) CheckAndSetDefaults()"). Packages often
	// have many functions/methods sharing the same name (e.g. dozens of
	// CheckAndSetDefaults implementations in api/types); Receiver
	// disambiguates between them. If empty, matches any function/method
	// with the given name, regardless of receiver - this is the original
	// behavior, unchanged.
	Receiver string
}

// Targets returns the packages and functions this linter checks.
func Targets() []Target {
	return []Target{
		{
			Package:  "github.com/gravitational/teleport/api/types",
			Function: "checkMCPStdio",
		},
		{
			Package:  "github.com/gravitational/teleport/lib/tbot/config",
			Function: "CheckAndSetDefaults",
		},
		{
			Package:  "github.com/gravitational/teleport/api/types",
			Function: "CheckAndSetDefaults",
			Receiver: "ProvisionTokenV2",
		},
	}
}

// ErrorConstructors returns the error-constructing functions this linter
// looks for within any in-scope function returned by Targets.
func ErrorConstructors() []Target {
	return []Target{
		{
			Package:  "github.com/gravitational/trace",
			Function: "BadParameter",
		},
	}
}
