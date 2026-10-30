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

package saml

const (
	// EduPersonAffiliation maps to user roles.
	EduPersonAffiliation = "eduPersonAffiliation"
	// SessionAccessRequests maps to Access Request IDs that are present in the
	// user session's identity.ActiveRequests field.
	SessionAccessRequests = "session.access_requests"
	// UID maps to username.
	UID = "uid"
	// Username maps to user resource name.
	Username = "user.metadata.name"
	// UserRoles maps to user roles.
	UserRoles = "user.spec.roles"
	// UserTraits maps to user traits.
	UserTraits = "user.spec.traits"
)
