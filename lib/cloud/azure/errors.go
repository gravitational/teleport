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
		switch responseErr.StatusCode {
		case http.StatusForbidden:
			return trace.AccessDenied("%s", responseErr)
		case http.StatusConflict:
			return trace.AlreadyExists("%s", responseErr)
		case http.StatusNotFound:
			return trace.NotFound("%s", responseErr)
		case http.StatusTooManyRequests:
			// TODO(marco): Extract the Retry-After header or any other indication of when we can retry the request.
			return trace.LimitExceeded("%s", responseErr)

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
