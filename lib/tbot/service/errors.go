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
package service

import "errors"

// IrrecoverableError wraps the given error to mark it as irrecoverable, so the
// supervisor will shut down all services.
func IrrecoverableError(err error) error {
	if err == nil {
		return nil
	}
	return irrecoverableError{err}
}

type irrecoverableError struct{ inner error }

func (e irrecoverableError) Error() string { return e.inner.Error() }
func (e irrecoverableError) Unwrap() error { return e.inner }

// IsIrrecoverableError returns whether the given error has been wrapped with
// IrrecoverableError.
func IsIrrecoverableError(err error) bool { return errors.As(err, &irrecoverableError{}) }
