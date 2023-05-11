// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lib

import (
	"context"
	"errors"
	"io"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TODO: remove this when trail.FromGRPC will understand additional error codes
func FromGRPC(err error) error {
	switch {
	case errors.Is(err, io.EOF):
		fallthrough
	case status.Code(err) == codes.Canceled, err == context.Canceled:
		fallthrough
	case status.Code(err) == codes.DeadlineExceeded, err == context.DeadlineExceeded:
		return trace.Wrap(err)
	default:
		return trail.FromGRPC(err)
	}
}

// TODO: remove this when trail.FromGRPC will understand additional error codes
func IsCanceled(err error) bool {
	err = trace.Unwrap(err)
	return err == context.Canceled || status.Code(err) == codes.Canceled
}

// TODO: remove this when trail.FromGRPC will understand additional error codes
func IsDeadline(err error) bool {
	err = trace.Unwrap(err)
	return err == context.DeadlineExceeded || status.Code(err) == codes.DeadlineExceeded
}
