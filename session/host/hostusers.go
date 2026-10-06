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

package host

import "errors"

// UserOpts allow for customizing the resulting command for adding a new user.
type UserOpts struct {
	// UID a user should be created with. When empty, the UID is determined by the
	// useradd command.
	UID string
	// GID a user should be assigned to on creation. When empty, a group of the same name
	// as the user will be used.
	GID string
	// Home directory for a user. When empty, this will be the root directory to match
	// OpenSSH behavior.
	Home string
	// Shell that the user should use when logging in. When empty, the default shell
	// for the host is used (typically /usr/bin/sh).
	Shell string
}

var ErrInvalidSudoers = errors.New("visudo: invalid sudoers file")
