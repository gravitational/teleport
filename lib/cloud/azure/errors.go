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
		switch responseErr.StatusCode {
		case http.StatusForbidden:
			return trace.AccessDenied("%s", responseErr)
		case http.StatusConflict:
			return trace.AlreadyExists("%s", responseErr)
		case http.StatusNotFound:
			return trace.NotFound("%s", responseErr)
		case http.StatusTooManyRequests:
			return wrapWithRetryAfterHeader(
				responseErr.RawResponse.Header,
				trace.LimitExceeded("%s", responseErr),
			)
		case http.StatusBadRequest:
			// Azure API return BadRequest in multiple scenarios, so we need to inspect the error details to ensure we return the most accurate error type.
			if errorDetails := errorDetails(responseErr); errorDetails != nil {
				switch errorDetails.Code {
				case "SubscriptionsContainInvalidGuids":
					return trace.BadParameter("%s", responseErr)

				case "NoValidSubscriptionsInQueryRequest":
					return trace.AccessDenied("%s", responseErr)
				}
			}
		}
		if responseErr.StatusCode >= http.StatusInternalServerError {
			return trace.BadParameter("azure api failed with status code %d: %s", responseErr.StatusCode, responseErr.Error())
		}
	case errors.As(err, &authenticationFailedErr):
		return trace.AccessDenied("%s", authenticationFailedErr)
	}
	return err // Return unmodified.
}

// errorDetails does a best-effort attempt to extract error details, and may return nil if the detail cannot be extracted for any reason.
func errorDetails(responseErr *azcore.ResponseError) *apiErrorDetail {
	if responseErr == nil || responseErr.RawResponse == nil {
		return nil
	}
	body := responseErr.RawResponse.Body
	defer body.Close()

	var apiErrResp apiResponseError
	if err := json.NewDecoder(body).Decode(&apiErrResp); err != nil {
		return nil
	}

	if len(apiErrResp.Error.Details) == 0 {
		return nil
	}

	return &apiErrResp.Error.Details[0]
}

type apiResponseError struct {
	Error struct {
		Details []apiErrorDetail `json:"details"`
	} `json:"error"`
}

type apiErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorFromResponse converts an HTTP response into an error.
// If the status code is not an error code, it returns nil.
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

// wrapWithRetryAfterHeader converts an API error into a [*RateLimitError] when the
// error is a limit-exceeded error, extracting the retry-after value from the
// supplied header.
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
		XMsUserQuotaResetsAfterHeader = "X-Ms-User-Quota-Resets-After"
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
	timeParts := strings.Split(header.Get(XMsUserQuotaResetsAfterHeader), ":")
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
