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

package lib

import (
	"context"
	"errors"
	"io"

	"github.com/gravitational/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport/api/trail"
)

// TODO: remove this when trail.FromGRPC will understand additional error codes
func FromGRPC(err error) error {
	switch {
	case errors.Is(err, io.EOF):
		fallthrough
	case status.Code(err) == codes.Canceled, errors.Is(err, context.Canceled):
		fallthrough
	case status.Code(err) == codes.DeadlineExceeded, errors.Is(err, context.DeadlineExceeded):
		return trace.Wrap(err)
	default:
		return trail.FromGRPC(err)
	}
}

// TODO: remove this when trail.FromGRPC will understand additional error codes
func IsCanceled(err error) bool {
	err = trace.Unwrap(err)
	return errors.Is(err, context.Canceled) || status.Code(err) == codes.Canceled
}

// TODO: remove this when trail.FromGRPC will understand additional error codes
func IsDeadline(err error) bool {
	err = trace.Unwrap(err)
	return errors.Is(err, context.DeadlineExceeded) || status.Code(err) == codes.DeadlineExceeded
}
