/*
Copyright 2016 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package csrf

import (
	"crypto/subtle"
	"encoding/hex"
	"net/http"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

const (
	// CookieName is a name of the cookie
	CookieName = "grv_csrf"
	// HeaderName is the default HTTP request header to inspect
	HeaderName = "X-CSRF-Token"
	// tokenLenBytes is CSRF token length in bytes.
	tokenLenBytes = 32
	// defaultMaxAge is the default MaxAge for cookies.
	defaultMaxAge = 0
)

// AddCSRFProtection adds CSRF token into the user session via secure cookie,
// it implements "double submit cookie" approach to check against CSRF attacks
// https://www.owasp.org/index.php/Cross-Site_Request_Forgery_%28CSRF%29_Prevention_Cheat_Sheet#Double_Submit_Cookie
func AddCSRFProtection(w http.ResponseWriter, r *http.Request) (string, error) {
	encodedToken := ""
	token, err := extractFromCookie(r)
	// if there was an error retrieving the token, the token doesn't exist
	if err != nil || len(token) == 0 {
		encodedToken, err = utils.CryptoRandomHex(tokenLenBytes)
		if err != nil {
			return "", trace.Wrap(err)
		}
	} else {
		encodedToken = hex.EncodeToString(token)
	}

	save(encodedToken, w)
	return encodedToken, nil
}

// VerifyToken checks if the cookie value and request value match.
func VerifyToken(w http.ResponseWriter, r *http.Request) error {
	realToken, err := extractFromCookie(r)
	if err != nil {
		return trace.BadParameter("cannot retrieve CSRF token from cookie", err)
	}

	if len(realToken) != tokenLenBytes {
		return trace.BadParameter("invalid CSRF cookie token length, expected %v, got %v", tokenLenBytes, len(realToken))
	}

	requestToken, err := extractFromRequest(r)
	if err != nil {
		return trace.BadParameter("cannot retrieve CSRF token from HTTP header", err)
	}

	// compare the request token against the real token
	if !compareTokens(requestToken, realToken) {
		return trace.BadParameter("request and cookie CSRF tokens do not match")
	}

	return nil
}

// extractFromCookie retrieves a CSRF token from the session cookie.
func extractFromCookie(r *http.Request) ([]byte, error) {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := decode(cookie.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return token, nil
}

// extractFromRequest returns the issued token from HTTP header.
func extractFromRequest(r *http.Request) ([]byte, error) {
	issued := r.Header.Get(HeaderName)
	decoded, err := decode(issued)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return decoded, nil
}

// save stores encoded CSRF token in the session cookie.
func save(encodedToken string, w http.ResponseWriter) string {
	cookie := &http.Cookie{
		Name:     CookieName,
		Value:    encodedToken,
		MaxAge:   defaultMaxAge,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
	}

	// write the authenticated cookie to the response.
	http.SetCookie(w, cookie)
	w.Header().Add("Vary", "Cookie")
	return encodedToken
}

// compare securely (constant-time) compares request token against the real token
// from the session.
func compareTokens(a, b []byte) bool {
	// this is required as subtle.ConstantTimeCompare does not check for equal
	// lengths in Go versions prior to 1.3.
	if len(a) != len(b) {
		return false
	}

	return subtle.ConstantTimeCompare(a, b) == 1
}

// decode decodes a cookie using base64.
func decode(value string) ([]byte, error) {
	return hex.DecodeString(value)
}
