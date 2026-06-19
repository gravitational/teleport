// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package errors

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gravitational/trace"
)

var (
	// ErrTimeout returned when the request times out.
	ErrTimeout = errors.New("the request timed out. Try again or use streaming for long responses")
	// ErrBadRequest returned when the request has bad format or invalid fields.
	ErrBadRequest = errors.New("the inference provider rejected the request as invalid. Check the request body for unsupported or invalid fields")
	// ErrCanceled returned when the request is canceled.
	ErrCanceled = errors.New("the request was canceled")
	// ErrUnauthorized returned when the request is unauthorized.
	ErrUnauthorized = errors.New("the inference provider rejected the request due to authentication or authorization configuration. Contact your Teleport administrator")
	// ErrRejected returned when the provider rejects the request.
	ErrRejected = errors.New("the inference provider rejected the request due to usage limits. Contact your Teleport administrator")
	// ErrUnsupported returned when the requested endpoint is not supported.
	ErrUnsupported = errors.New("teleport doesn't support the requested endpoint, please check the list of supported endpoints in the documentation")
	// ErrBadResponse returned when the provider replied the request with an unsupported message or format.
	ErrBadResponse = errors.New("the inference provider returned an unexpected response. Contact your Teleport administrator")
	// ErrUnknown returned when the handler could not identify the error.
	ErrUnknown = errors.New("the inference provider returned an unexpected error. Contact your Teleport administrator")
)

// ProviderError is an error in the provider format.
type ProviderError struct {
	err    error
	detail string
}

// NewProviderError creates a new provider error with details.
func NewProviderError(err error, detail string, args ...any) *ProviderError {
	if len(args) > 0 {
		detail = fmt.Sprintf(detail, args...)
	}
	return &ProviderError{err, detail}
}

func (e *ProviderError) Error() string {
	return e.UserMessage()
}

func (e *ProviderError) Unwrap() error {
	return e.err
}

func (e *ProviderError) UserMessage() string {
	if e.detail != "" {
		return e.err.Error() + ": " + e.detail
	}
	return e.err.Error()
}

// StatusCodeFromErr returns HTTP status code from error.
//
// When cannot fully rely on `trace.ErrorToCode` because LLM endpoints require
// a more granular error codes than what is provided by `trace`.
func StatusCodeFromErr(err error) int {
	switch {
	case errors.Is(err, ErrCanceled), errors.Is(err, ErrTimeout):
		return http.StatusGatewayTimeout
	case errors.Is(err, ErrBadRequest), errors.Is(err, ErrBadResponse):
		return http.StatusBadRequest
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrRejected):
		return http.StatusTooManyRequests
	case errors.Is(err, ErrUnsupported):
		return http.StatusNotFound
	default:
		return trace.ErrorToCode(err)
	}
}
