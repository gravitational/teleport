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

// VerifyHTTPHeader checks if HTTP header value matches the cookie.
func VerifyHTTPHeader(r *http.Request) error {
	token := r.Header.Get(HeaderName)
	if len(token) == 0 {
		return trace.BadParameter("cannot retrieve CSRF token from HTTP header %q", HeaderName)
	}

	err := VerifyToken(token, r)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
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

	if !compareTokens(decodedTokenA, decodedTokenB) {
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

// decode decodes a cookie using base64.
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

// compareTokens securely (constant-time) compares CSRF tokens
func compareTokens(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
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
