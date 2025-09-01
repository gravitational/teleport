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

package accesslists

import (
	"errors"

	"github.com/gravitational/trace"
)

// userLockedError is used to check specific condition of user being locked with [IsUserLocked]. It
// is also being matched by [trace.IsAccessDenied] while allowing creating a dynamic error message
// containing the username.
type userLockedError struct{ err error }

// newUserLockedError returns a new userLockedError.
func newUserLockedError(user string) userLockedError {
	return userLockedError{trace.AccessDenied("User %q is currently locked", user)}
}

func (e userLockedError) Unwrap() error { return e.err }
func (e userLockedError) Error() string { return e.err.Error() }

// IsUserLocked checks if the error was a result of the Access List member user having a lock.
func IsUserLocked(err error) bool {
	return errors.As(err, &userLockedError{})
}
