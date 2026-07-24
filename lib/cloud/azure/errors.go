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
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
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
			var header http.Header
			if responseErr.RawResponse != nil {
				header = responseErr.RawResponse.Header
			}
			return wrapWithRetryAfterHeader(
				header,
				trace.LimitExceeded("%s", responseErr),
			)
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
	if responseErr == nil || responseErr.RawResponse == nil || responseErr.RawResponse.Body == nil {
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

// ErrorFromResponse converts an HTTP response into an error.
func ErrorFromResponse(response *http.Response) error {
	return ConvertResponseError(runtime.NewResponseError(response))
}

// RateLimitError is returned by Azure API when the server signals a rate or concurrency limit.
// It wraps a [trace.LimitExceededError] and carries the retry-after duration extracted from the API response.
type RateLimitError struct {
	// RetryAfter is the value of the "retry-after" header, or 0 if the header was absent.
	RetryAfter time.Duration
	// Err is the underlying LimitExceeded trace error.
	Err error
}

// Error returns the underlying error message.
func (e *RateLimitError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the underlying error, allowing errors.Is and errors.As to work with RateLimitError.
func (e *RateLimitError) Unwrap() error {
	return e.Err
}

// wrapWithRetryAfterHeader wraps apiErr in a [*RateLimitError], setting its
// retry-after value from the supplied header. Callers are responsible for only
// invoking it with a limit-exceeded error.
func wrapWithRetryAfterHeader(header http.Header, apiErr error) error {
	return &RateLimitError{
		Err:        apiErr,
		RetryAfter: extractRetryAfterDuration(header),
	}
}

// extractRetryAfterDuration extracts the retry-after duration as documented by Azure.
// When it is not present or cannot be parsed, it returns 0.
func extractRetryAfterDuration(header http.Header) time.Duration {
	const (
		retryAfterHeader              = "Retry-After"
		xMsUserQuotaResetsAfterHeader = "X-Ms-User-Quota-Resets-After"
	)
	// See https://learn.microsoft.com/en-us/azure/azure-resource-manager/management/request-limits-and-throttling#error-code
	// > When you reach the limit, you receive the HTTP status code 429 Too many requests.
	// > The response includes a Retry-After value, which specifies the number of seconds your application should wait before sending the next request.
	retryAfterSeconds, err := strconv.Atoi(header.Get(retryAfterHeader))
	if err == nil {
		return time.Duration(retryAfterSeconds) * time.Second
	}

	// Specifically for Azure Resource Graph, the API can also return the "X-Ms-User-Quota-Resets-After" header.
	// See https://learn.microsoft.com/en-us/azure/governance/resource-graph/concepts/guidance-for-throttled-requests#understand-throttling-headers
	// This comes in the following format: X-Ms-User-Quota-Resets-After: 00:00:05
	timeParts := strings.Split(header.Get(xMsUserQuotaResetsAfterHeader), ":")
	if len(timeParts) == 3 {
		hours, err1 := strconv.Atoi(timeParts[0])
		minutes, err2 := strconv.Atoi(timeParts[1])
		seconds, err3 := strconv.Atoi(timeParts[2])
		if err1 == nil && err2 == nil && err3 == nil {
			return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
		}
	}

	return 0
}
