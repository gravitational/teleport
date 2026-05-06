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

package llm

import (
	"context"
	"errors"
	"net/http"

	"github.com/gravitational/trace"
)

// errKind is an internal kind used by provider formatters to map an
// apiError to the corresponding error type.
type errKind int

const (
	// errKindUnknown means the error could not be classified.
	errKindUnknown errKind = iota
	// errKindCanceled means the request was canceled by the caller.
	errKindCanceled
	// errKindTimeout means the request exceeded its deadline.
	errKindTimeout
	// errKindBadRequest means the provider rejected the request as invalid.
	errKindBadRequest
	// errKindUnauthorized means the provider rejected the request due to
	// authentication or authorization.
	errKindUnauthorized
	// errKindRejected means the provider rejected the request due to usage
	// limits or billing constraints.
	errKindRejected
	// errKindUnsupportedEndpoint means Teleport doesn't support the requested
	// endpoint.
	errKindUnsupportedEndpoint
)

// apiError is a sanitized error suitable for forwarding to clients in any of
// the supported provider formats. It captures the original cause, an HTTP
// status, and a user-facing message. Provider-specific code translates these
// into the corresponding wire format using kind for dispatch.
type apiError struct {
	cause   error
	kind    errKind
	status  int
	message string
	detail  string
}

func (e *apiError) Error() string {
	return e.UserMessage()
}

func (e *apiError) Unwrap() error {
	return e.cause
}

// StatusCode returns the HTTP status code to send to the client.
func (e *apiError) StatusCode() int {
	return e.status
}

func (e *apiError) UserMessage() string {
	if e.detail != "" {
		return e.message + ": " + e.detail
	}
	return e.message
}

func newCanceled(cause error) *apiError {
	return &apiError{
		cause:   cause,
		kind:    errKindCanceled,
		status:  http.StatusGatewayTimeout,
		message: "The request was canceled",
	}
}

func newTimeout(cause error, status int) *apiError {
	return &apiError{
		cause:   cause,
		kind:    errKindTimeout,
		status:  status,
		message: "The request timed out. Try again or use streaming for long responses",
	}
}

func newBadRequest(cause error, status int, detail string) *apiError {
	return &apiError{
		cause:   cause,
		kind:    errKindBadRequest,
		status:  status,
		message: "The inference provider rejected the request as invalid. Check the request body for unsupported or invalid fields",
		detail:  detail,
	}
}

func newUnauthorized(cause error, status int) *apiError {
	return &apiError{
		cause:   cause,
		kind:    errKindUnauthorized,
		status:  status,
		message: "The inference provider rejected the request due to authentication or authorization configuration. Contact your Teleport administrator",
	}
}

func newRejected(cause error, status int) *apiError {
	return &apiError{
		cause:   cause,
		kind:    errKindRejected,
		status:  status,
		message: "The inference provider rejected the request due to usage limits. Contact your Teleport administrator",
	}
}

func newUnsupportedEndpoint(cause error) *apiError {
	return &apiError{
		cause:   cause,
		kind:    errKindUnsupportedEndpoint,
		status:  http.StatusNotFound,
		message: "Teleport doesn't support the requested endpoint, please check the list of supported endpoints on our documentation.",
	}
}

func newUnknown(cause error, status int) *apiError {
	if status == 0 {
		status = http.StatusBadGateway
	}
	return &apiError{
		cause:   cause,
		kind:    errKindUnknown,
		status:  status,
		message: "The inference provider returned an unexpected error. Contact your Teleport administrator",
	}
}

// convertError handles errors that aren't tied to any specific provider
// (context cancellation, trace errors thrown by validators).
func convertError(err error) *apiError {
	switch {
	case errors.Is(err, context.Canceled):
		return newCanceled(err)
	case errors.Is(err, context.DeadlineExceeded):
		return newTimeout(err, http.StatusGatewayTimeout)
	case trace.IsBadParameter(err):
		return newBadRequest(err, http.StatusBadRequest, err.Error())
	case trace.IsNotFound(err):
		return newUnsupportedEndpoint(err)
	}
	return nil
}

// convertErrorByHTTPStatus produces an apiError from an HTTP status code, used
// when a provider returns an error without a parseable body.
func convertErrorByHTTPStatus(cause error, status int, message string) *apiError {
	if status == 0 {
		status = http.StatusBadGateway
	}
	switch status {
	case http.StatusGatewayTimeout, http.StatusRequestTimeout:
		return newTimeout(cause, status)
	case http.StatusUnauthorized, http.StatusForbidden:
		return newUnauthorized(cause, status)
	case http.StatusBadRequest:
		return newBadRequest(cause, status, message)
	case http.StatusPaymentRequired, http.StatusTooManyRequests:
		return newRejected(cause, status)
	}
	return newUnknown(cause, status)
}
