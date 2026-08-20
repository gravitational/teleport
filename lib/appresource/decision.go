/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package appresource

// DenyKind is the category of a denial, emitted as deny_kind on the
// app.session.request.denied audit event.
type DenyKind string

const (
	// DenyNotAllowed is the kind for a well-formed request that no allow
	// rule matched.
	DenyNotAllowed DenyKind = "teleport_request_not_allowed"
	// DenyRoleVersionUnsupported is the kind for a request denied because a
	// role or a rule is a version this agent cannot evaluate, as in a
	// mixed-version cluster.
	DenyRoleVersionUnsupported DenyKind = "teleport_role_version_unsupported"
)
