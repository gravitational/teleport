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

package img

import (
	"fmt"

	"github.com/distribution/reference"
)

// ImageNotValidatedError indicates that the validator did not validate the image.
// It wraps the original error and implements trace.ErrorWrapper.
type ImageNotValidatedError struct {
	err           error
	image         reference.NamedTagged
	validatorName string
}

// Error returns log friendly description of an error.
func (e *ImageNotValidatedError) Error() string {
	return fmt.Sprintf("validator '%s' rejected the image '%s': %s", e.validatorName, e.image, e.err.Error())
}

// OrigError return the original error.
func (e *ImageNotValidatedError) OrigError() error {
	return e.err
}

// Unwrap unwraps the error and return the original error.
func (e *ImageNotValidatedError) Unwrap() error {
	return e.err
}

// NotValidatedError returns an ImageNotValidatedError if the error is not nil.
func NotValidatedError(name string, image reference.NamedTagged, err error) *ImageNotValidatedError {
	if err == nil {
		return nil
	}
	return &ImageNotValidatedError{err, image, name}
}
