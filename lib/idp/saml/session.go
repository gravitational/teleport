/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package saml

import (
	"net/http"
)

const (
	// SAMLSessionCookieName is the name of the SAML IdP session cookie.
	// This cookie is set by SAML IdP after a successful SAML authentication.
	SAMLSessionCookieName = "__Host-saml_session"
)

// SetCookie set's the SAML session cookie named by [SAMLSessionCookieName].
func SetCookie(w http.ResponseWriter, sessionID string, maxAgeSeconds int) {
	http.SetCookie(w, &http.Cookie{
		Name:     SAMLSessionCookieName,
		Value:    sessionID,
		MaxAge:   maxAgeSeconds,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
	})
}

// ClearCookie wipes the session cookie to invalidate SAML user session.
func ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SAMLSessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
	})
}
