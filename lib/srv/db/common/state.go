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

package common

// Teleport-related SQL states.
//
// SQLSTATE reference:
// https://en.wikipedia.org/wiki/SQLSTATE
const (
	// SQLStateActiveUser is the SQLSTATE raised by deactivation procedure when
	// user has active connections.
	SQLStateActiveUser = "TP000"
	// SQLStateUsernameDoesNotMatch is the SQLSTATE raised by activation
	// procedure when the Teleport username does not match user's attributes.
	//
	// Possibly there is a hash collision, or someone manually updated the user
	// attributes.
	SQLStateUsernameDoesNotMatch = "TP001"
	// SQLStateRolesChanged is the SQLSTATE raised by activation procedure when
	// the user has active connections but roles have changed.
	SQLStateRolesChanged = "TP002"
	// SQLStateUserDropped is the SQLSTATE returned by the delete procedure
	// indicating the user was dropped.
	SQLStateUserDropped = "TP003"
	// SQLStateUserDeactivated is the SQLSTATE returned by the delete procedure
	// indicating was deactivated.
	SQLStateUserDeactivated = "TP004"
	// SQLStatePermissionsChanged is the SQLSTATE raised by permissions update procedure when
	// the user has active connections for current database but permissions have changed.
	SQLStatePermissionsChanged = "TP005"
)
