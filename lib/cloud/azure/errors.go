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
			return trace.AccessDenied(responseErr.Error())
		case http.StatusConflict:
			return trace.AlreadyExists(responseErr.Error())
		case http.StatusNotFound:
			return trace.NotFound(responseErr.Error())
		}
	case errors.As(err, &authenticationFailedErr):
		return trace.AccessDenied(authenticationFailedErr.Error())
	}
	return err // Return unmodified.
}
