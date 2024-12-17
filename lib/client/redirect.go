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

package client

import "github.com/gravitational/teleport/lib/client/sso"

// TODO: delete once e ref is updated.
const (
	// LoginFailedRedirectURL is the default redirect URL when an SSO error was encountered.
	LoginFailedRedirectURL = sso.LoginFailedRedirectURL

	// LoginFailedBadCallbackRedirectURL is a redirect URL when an SSO error specific to
	// auth connector's callback was encountered.
	LoginFailedBadCallbackRedirectURL = sso.LoginFailedBadCallbackRedirectURL

	// LoginFailedUnauthorizedRedirectURL is a redirect URL for when an SSO authenticates successfully,
	// but the user has no matching roles in Teleport.
	LoginFailedUnauthorizedRedirectURL = sso.LoginFailedUnauthorizedRedirectURL

	// SAMLSingleLogoutFailedRedirectURL is the default redirect URL when an error was encountered during SAML Single Logout.
	SAMLSingleLogoutFailedRedirectURL = sso.SAMLSingleLogoutFailedRedirectURL

	// DefaultLoginURL is the default login page.
	DefaultLoginURL = sso.DefaultLoginURL
)
