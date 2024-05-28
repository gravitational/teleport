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

package cache

import (
	"fmt"
	"time"
)

// cachedError wraps an error before storing it into cache. This adds more
// context into the original error by clearly indicating the error have been
// cached and for how long.
type cachedError struct {
	err   error
	until time.Time
}

func (e cachedError) Error() string {
	return fmt.Sprintf("error cached until '%s': %s", e.until, e.err)
}

// OrigError returns the original error. This implements trace.ErrorWrapper
// and allows to be unwrapped by trace.Unwrap().
func (e cachedError) OrigError() error {
	return e.err
}

// Unwrap returns the original error.
func (e cachedError) Unwrap() error {
	return e.err
}

// newCachedError takes an error and wraps it into a cachedError. If there is no
// error, it returns nothing.
func newCachedError(err error, until time.Time) error {
	if err == nil {
		return nil
	}
	return cachedError{err, until}
}
