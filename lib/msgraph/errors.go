// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package msgraph

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
)

// unsupportedGroupMember is an internal error to indicate that
// the `groupmembers` endpoint has returned a member of type that we do not support (yet).
type unsupportedGroupMember struct {
	Type string
}

func (u *unsupportedGroupMember) Error() string {
	return fmt.Sprintf("Unsupported group member: %q", u.Type)
}

type graphErrorResponse struct {
	Error *GraphError `json:"error,omitempty"`
}

// GraphError defines the structure of errors returned from MS Graph API.
// https://learn.microsoft.com/en-us/graph/errors#json-representation
type GraphError struct {
	// Code is the code for the error, e.g. "UnknownError", "BadRequest".
	Code string `json:"code,omitempty"`
	// Message is a developer ready message about the error that occurred. This shouldn't be displayed
	// to the user directly.
	Message string `json:"message,omitempty"`
	// InnerError is an optional additional error object that is more specific than the top-level
	// error.
	InnerError *GraphError `json:"innerError,omitempty"`
	// Details is an optional list of more error objects that provide a breakdown of multiple errors
	// encountered while processing the request.
	Details []GraphError `json:"details,omitempty"`
	// StatusCode is the status code of the HTTP response that GraphError arrived with.
	StatusCode int `json:"-"`
}

func (g *GraphError) Error() string {
	var parts []string
	if g.Code != "" {
		parts = append(parts, strings.TrimPrefix(g.Code, "Request_"))
	}

	if g.Message != "" {
		parts = append(parts, g.Message)
	}

	return strings.Join(parts, ": ")
}

func readError(body []byte, statusCode int) (*GraphError, error) {
	var errResponse graphErrorResponse
	if err := json.Unmarshal(body, &errResponse); err != nil {
		return nil, trace.Wrap(err)
	}
	if errResponse.Error == nil {
		return nil, nil
	}
	graphError := errResponse.Error
	graphError.StatusCode = statusCode
	return graphError, nil
}

// AuthError is the error returned by [AzureTokenProvider.GetToken] indicating that an
// authentication request has failed.
// https://learn.microsoft.com/en-us/entra/identity-platform/reference-error-codes
type AuthError struct {
	// ErrorCode is the code string for the error, e.g. "invalid_client".
	ErrorCode string `json:"error"`
	// ErrorDescription is a specific error message that can help a developer identify the root cause
	// of an authentication error.
	ErrorDescription string `json:"error_description"`
	// DiagCodes is a list of codes used by the security token service which map to specific reasons
	// as to why a request have failed.
	// https://learn.microsoft.com/en-us/entra/identity-platform/reference-error-codes#aadsts-error-codes
	DiagCodes  []int `json:"error_codes"`
	StatusCode int   `json:"-"`
}

func (a *AuthError) Error() string {
	var b strings.Builder
	b.WriteString(a.ErrorDescription)
	b.WriteString(" (")
	b.WriteString(a.ErrorCode)
	if len(a.DiagCodes) > 0 {
		if len(a.DiagCodes) == 1 {
			b.WriteString(", diag code ")
		} else {
			b.WriteString(", diag codes ")
		}
		for i, errorCode := range a.DiagCodes {
			if i != 0 {
				b.WriteString(", ")
			}
			b.WriteString(strconv.Itoa(errorCode))
		}
	}
	b.WriteString(")")
	return b.String()
}

func readAuthError(r io.Reader, statusCode int) (*AuthError, error) {
	var authError AuthError
	authError.StatusCode = statusCode
	err := json.NewDecoder(r).Decode(&authError)
	return &authError, trace.Wrap(err)
}

const (
	// DiagCodeTenantNotFound is returned by the identity platform when the specific tenant doesn't
	// exist. It might also mean that the tenant belongs to another cloud (see
	// https://learn.microsoft.com/en-us/graph/deployments) or there are no active subscriptions for
	// the tenant.
	// https://login.microsoftonline.com/error?code=90002
	DiagCodeTenantNotFound = 90002
	// DiagCodeInvalidTenantIdentifier is returned by the identity platform when the identifier is
	// neither a valid DNS name nor a valid external domain. This happes when the tenant is not a UUID
	// and instead a regular string that doesn't match said requirements.
	// https://login.microsoftonline.com/error?code=900023
	DiagCodeInvalidTenantIdentifier = 900023
)
