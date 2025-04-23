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

// Package csrf is used to protect against login CSRF in Teleport's SSO flows.
package csrf

import (
	"crypto/subtle"
	"encoding/hex"
	"net/http"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

// CookieName is the name of the CSRF cookie used to protect against login
// CSRF in Teleport's SSO flows.
//
// It's prefixed with "__Host-" as an additional defense in depth measure.
// This makes sure the cookie is sent from a secure page (HTTPS),
// won't be sent to subdomains, and the path attribute is set to /.
const CookieName = "__Host-grv_csrf"

// tokenLenBytes is the length of a raw CSRF token prior to encoding
const tokenLenBytes = 32

// AddCSRFProtection adds CSRF token into the user session via a secure cookie.
// This CSRF token is used to protect against login CSRF in Teleport's SSO flows.
func AddCSRFProtection(w http.ResponseWriter, r *http.Request) (string, error) {
	token, err := ExtractTokenFromCookie(r)
	// if there was an error retrieving the token, the token doesn't exist
	if err != nil || len(token) == 0 {
		token, err = utils.CryptoRandomHex(tokenLenBytes)
		if err != nil {
			return "", trace.Wrap(err)
		}
	}
	save(token, w)
	return token, nil
}

// VerifyToken validates given token based on HTTP request cookie
func VerifyToken(token string, r *http.Request) error {
	realToken, err := ExtractTokenFromCookie(r)
	if err != nil {
		return trace.Wrap(err, "unable to extract CSRF token from cookie")
	}

	decodedTokenA, err := decode(token)
	if err != nil {
		return trace.Wrap(err, "unable to decode CSRF token")
	}

	decodedTokenB, err := decode(realToken)
	if err != nil {
		return trace.Wrap(err, "unable to decode cookie CSRF token")
	}

	if subtle.ConstantTimeCompare(decodedTokenA, decodedTokenB) != 1 {
		return trace.BadParameter("CSRF tokens do not match")
	}

	return nil
}

// ExtractTokenFromCookie retrieves a CSRF token from the session cookie.
func ExtractTokenFromCookie(r *http.Request) (string, error) {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return cookie.Value, nil
}

// decode decodes a hex-encoded CSRF token.
func decode(token string) ([]byte, error) {
	decoded, err := hex.DecodeString(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(decoded) != tokenLenBytes {
		return nil, trace.BadParameter("invalid CSRF token byte length, expected %v, got %v", tokenLenBytes, len(decoded))
	}

	return decoded, nil
}

// save stores encoded CSRF token in the session cookie.
func save(encodedToken string, w http.ResponseWriter) string {
	cookie := &http.Cookie{
		Name:  CookieName,
		Value: encodedToken,
		// Set SameSite to none so browsers preserve gravitational CSRF cookie
		// while processing SSO providers redirects.
		SameSite: http.SameSiteNoneMode,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
	}

	// write the authenticated cookie to the response.
	http.SetCookie(w, cookie)
	w.Header().Add("Vary", "Cookie")
	return encodedToken
}
