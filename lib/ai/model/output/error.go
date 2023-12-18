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

package output

import (
	"fmt"

	"github.com/gravitational/trace"
)

// NewInvalidOutputError builds an error caused by the output of an LLM.
func NewInvalidOutputError(coarse, detail string) error {
	return &invalidOutputError{
		coarse: coarse,
		detail: detail,
	}
}

// IsInvalidOutputError returns true if the error is an invalidOutputError.
func IsInvalidOutputError(err error) bool {
	_, ok := trace.Unwrap(err).(*invalidOutputError)
	return ok
}

// invalidOutputError represents an error caused by the output of an LLM.
// These may be used automatically by the agent loop to attempt to correct an output until it is valid.
type invalidOutputError struct {
	coarse string
	detail string
}

// newInvalidOutputErrorWithParseError creates a new invalidOutputError assuming a JSON parse error.
func newInvalidOutputErrorWithParseError(err error) *invalidOutputError {
	return &invalidOutputError{
		coarse: "json parse error",
		detail: err.Error(),
	}
}

// Error returns a string representation of the error. This is used to satisfy the error interface.
func (o *invalidOutputError) Error() string {
	return fmt.Sprintf("%v: %v", o.coarse, o.detail)
}
