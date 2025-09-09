// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

// VerifyCredentials checks if the credentials supplied to the client can be used to authenticate to
// the Graph API and if they're authorized to get specific resources.
package msgraph

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/gravitational/trace"
)

var (
	// ErrTenantNotFound is returned by [Client.VerifyCredentials] when getting a token fails due to
	// the tenant not being found. It might also point to the subscription no longer being active.
	ErrTenantNotFound = errors.New("tenant not found")
	// ErrInvalidCredentials is returned by [Client.VerifyCredentials] when getting a token fails due
	// to an invalid client ID or secret.
	ErrInvalidCredentials = errors.New("invalid Graph API credentials")
	// ErrClientUnauthorized is returned by [Client.VerifyCredentials] in a situation where the app
	// either doesn't have the permission required to access certain resources or the permission
	// hasn't been grated by the administrator yet.
	ErrClientUnauthorized = errors.New("authentication was successful but application does not have necessary permissions")
)

// IsCredentialsError determines whether err is one of the special errors returned by
// [Client.VerifyCredentials].
func IsCredentialsError(err error) bool {
	return errors.Is(err, ErrTenantNotFound) ||
		errors.Is(err, ErrInvalidCredentials) ||
		errors.Is(err, ErrClientUnauthorized)
}

// VerifyCredentials expects getResourcesFunc to call a method on [Client]. It then inspects the
// returned error to check for Graph or token errors related to credentials being insufficient in
// some way.
func (c *Client) VerifyCredentials(ctx context.Context, getResourcesFunc func(ctx context.Context, client *Client) error) error {
	err := getResourcesFunc(ctx, c)

	graphError := &GraphError{}
	isGraphError := errors.As(err, &graphError)
	authError := &AuthError{}
	isAuthError := errors.As(err, &authError)

	switch {
	case isAuthError &&
		(slices.Contains(authError.DiagCodes, DiagCodeTenantNotFound) ||
			slices.Contains(authError.DiagCodes, DiagCodeInvalidTenantIdentifier)):
		return trace.Wrap(ErrTenantNotFound, authError.ErrorDescription)

	case isAuthError && authError.StatusCode == http.StatusBadRequest &&
		authError.ErrorCode == "unauthorized_client":
		// It likely means that the provided client ID doesn't exist under this tenant.
		// https://login.microsoftonline.com/error?code=700016
		return trace.Wrap(ErrInvalidCredentials, authError.ErrorDescription)

	case isAuthError && authError.StatusCode == http.StatusUnauthorized:
		// It likely means that the provided client secret doesn't match the client ID.
		// https://login.microsoftonline.com/error?code=7000215
		return trace.Wrap(ErrInvalidCredentials, authError.ErrorDescription)

	case isGraphError && graphError.StatusCode == http.StatusUnauthorized:
		// In this situation, the Graph API returns a very verbose and unhelpful error, hence why it's
		// caught here and a more specific error is returned. On the off chance that the Graph API
		// starts returning a more helpful message, let's also log it.
		c.logger.WarnContext(ctx, "Graph API status unauthorized", "error", err)
		return trace.Wrap(ErrClientUnauthorized)

	default:
		return trace.Wrap(err, "verifying Graph API credentials")
	}
}
