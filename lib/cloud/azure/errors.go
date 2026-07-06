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

package azure

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/trace"
)

// ConvertResponseError converts `error` into Azure Response error.
// to trace error. If the provided error is not a `ResponseError` it returns.
// the error without modifying it.
func ConvertResponseError(err error) error {
	if err == nil {
		return nil
	}

	var responseErr *azcore.ResponseError
	var authenticationFailedErr *azidentity.AuthenticationFailedError
	switch {
	case errors.As(err, &responseErr):
		apiResponseErr := apiErrorFromResponse(responseErr)

		switch responseErr.StatusCode {
		case http.StatusForbidden:
			return trace.AccessDenied("%s", responseErr)
		case http.StatusConflict:
			switch {
			case isVMNotRunningError(apiResponseErr):
				return NewVMNotRunningError(responseErr)
			case isVMAgentNotAvailableError(apiResponseErr):
				return NewVMAgentNotAvailableError(responseErr)
			default:
				return trace.AlreadyExists("%s", responseErr)
			}
		case http.StatusNotFound:
			return trace.NotFound("%s", responseErr)
		case http.StatusTooManyRequests:
			// TODO(marco): Extract the Retry-After header or any other indication of when we can retry the request.
			return trace.LimitExceeded("%s", responseErr)

		case http.StatusBadRequest:
			// Azure API return BadRequest in multiple scenarios, so we need to inspect the error details to ensure we return the most accurate error type.
			if errorDetails := apiResponseErr.errorDetails(); errorDetails != nil {
				switch errorDetails.Code {
				case "SubscriptionsContainInvalidGuids":
					return trace.BadParameter("%s", responseErr)

				case "NoValidSubscriptionsInQueryRequest":
					return trace.AccessDenied("%s", responseErr)
				}
			}
		}
	case errors.As(err, &authenticationFailedErr):
		return trace.AccessDenied("%s", authenticationFailedErr)
	}
	return err // Return unmodified.
}

func apiErrorFromResponse(responseErr *azcore.ResponseError) *apiResponseError {
	if responseErr == nil || responseErr.RawResponse == nil {
		return nil
	}
	body := responseErr.RawResponse.Body
	defer body.Close()

	var apiErrResp apiResponseError
	if err := json.NewDecoder(body).Decode(&apiErrResp); err != nil {
		return nil
	}

	return &apiErrResp
}

type apiResponseError struct {
	Error struct {
		Code    string           `json:"code"`
		Message string           `json:"message"`
		Details []apiErrorDetail `json:"details"`
	} `json:"error"`
}

type apiErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (a *apiResponseError) errorDetails() *apiErrorDetail {
	if a == nil || len(a.Error.Details) == 0 {
		return nil
	}

	return &a.Error.Details[0]
}

func (a *apiResponseError) code() string {
	if a == nil {
		return ""
	}

	return a.Error.Code
}
func (a *apiResponseError) message() string {
	if a == nil {
		return ""
	}

	return a.Error.Message
}

const (
	operationNotAllowedCode = "OperationNotAllowed"
)

func isVMNotRunningError(apiErr *apiResponseError) bool {
	return apiErr.code() == operationNotAllowedCode && strings.Contains(apiErr.message(), "VM is not running")
}

// NewVMNotRunningError creates a new VMNotRunningError with the provided inner error.
func NewVMNotRunningError(inner error) *VMNotRunningError {
	return &VMNotRunningError{
		inner: inner,
	}
}

// VMNotRunningError is returned when an operation is attempted on a virtual machine that is not in a running state.
type VMNotRunningError struct {
	inner error
}

// Unwrap returns the underlying error that caused the VMNotRunningError.
func (e *VMNotRunningError) Unwrap() error {
	return e.inner
}

// Is checks if the target error is of type VMNotRunningError.
func (e *VMNotRunningError) Is(target error) bool {
	_, ok := target.(*VMNotRunningError)
	return ok
}

// Error returns a message indicating that the virtual machine is not running.
func (e *VMNotRunningError) Error() string {
	return "VM is not running"
}

func isVMAgentNotAvailableError(apiErr *apiResponseError) bool {
	return apiErr.code() == operationNotAllowedCode && strings.Contains(apiErr.message(), "extension operations are disallowed")
}

// NewVMAgentNotAvailableError creates a new VMAgentNotAvailableError with the provided inner error.
func NewVMAgentNotAvailableError(inner error) *VMAgentNotAvailableError {
	return &VMAgentNotAvailableError{
		inner: inner,
	}
}

// VMAgentNotAvailableError is returned when an operation is attempted on a virtual machine whose agent is not available.
type VMAgentNotAvailableError struct {
	inner error
}

// Unwrap returns the underlying error that caused the VMAgentNotAvailableError.
func (e *VMAgentNotAvailableError) Unwrap() error {
	return e.inner
}

// Is checks if the target error is of type VMAgentNotAvailableError.
func (e *VMAgentNotAvailableError) Is(target error) bool {
	_, ok := target.(*VMAgentNotAvailableError)
	return ok
}

// Error returns a message indicating that the virtual machine's agent is not available.
func (e *VMAgentNotAvailableError) Error() string {
	return "VM agent is not available"
}
