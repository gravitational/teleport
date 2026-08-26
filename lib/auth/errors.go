/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package auth

import (
	"errors"

	"github.com/gravitational/trace"
)

var (
	errDeleteRoleUser = errors.New("failed to delete a role that is still in use by a user, check the system server logs for more details")
	errDeleteRoleCA   = errors.New("failed to delete a role that is still in use by a certificate authority, check the system server logs for more details")
)

// IsRoleInUseError checks if an error indicates that a role cannot be deleted
// because it is currently in use by users, certificate authorities, or access lists.
func IsRoleInUseError(err error) bool { return errors.As(err, &roleInUseError{}) }

func newRoleInUseError(msg string, args ...any) error {
	return roleInUseError{
		err: trace.BadParameter(msg, args...),
	}
}

type roleInUseError struct{ err error }

func (e roleInUseError) Unwrap() error { return e.err }
func (e roleInUseError) Error() string { return e.err.Error() }
