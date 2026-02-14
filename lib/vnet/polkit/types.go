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

package polkit

import "github.com/godbus/dbus/v5"

// Subject describes the entity being authorized.
type Subject struct {
	// Kind is the subject kind, e.g. "system-bus-name".
	Kind string
	// Details are subject kind specific key/value pairs.
	Details map[string]dbus.Variant
}

// AuthorizationResult is the result of CheckAuthorization.
type AuthorizationResult struct {
	// Authorized is true if the subject is authorized for the action.
	Authorized bool
	// Challenge is true if the subject could be authorized after authentication.
	Challenge bool
	// Details contains extra result information.
	Details map[string]string
}
