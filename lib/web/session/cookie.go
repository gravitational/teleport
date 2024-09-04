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

package session

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
)

// Cookie stores information about active user and session
type Cookie struct {
	User string `json:"user"`
	SID  string `json:"sid"`
}

// EncodeCookie returns the string representation of a [Cookie]
// that should be used to store the user session in the cookies
// of a [http.ResponseWriter].
func EncodeCookie(user, sid string) (string, error) {
	bytes, err := json.Marshal(Cookie{User: user, SID: sid})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// DecodeCookie returns the [Cookie] from the provided string.
func DecodeCookie(b string) (*Cookie, error) {
	bytes, err := hex.DecodeString(b)
	if err != nil {
		return nil, err
	}
	var c Cookie
	if err := json.Unmarshal(bytes, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// SetCookie encodes the provided user and session id via [EncodeCookie]
// and then sets the [http.Cookie] of the provided [http.ResponseWriter].
func SetCookie(w http.ResponseWriter, user, sid string) error {
	d, err := EncodeCookie(user, sid)
	if err != nil {
		return err
	}
	c := &http.Cookie{
		Name:     CookieName,
		Value:    d,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
	}
	http.SetCookie(w, c)
	return nil
}

// ClearCookie wipes the session cookie to invalidate the user session.
func ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
	})
}

const (
	// CookieName is the name of the session cookie.
	CookieName = "__Host-session"
)

const (
	// SSOMFAToken is the name of the SAML IdP session cookie.
	// This cookie is set by SAML IdP after a successful SAML authentication.
	SSOMFAToken = "__Host-sso_mfa_token"
)

// Cookie stores information about active user and session
type SSOMFACookie struct {
	RequestID string `json:"request_id"`
	MFAToken  string `json:"mfa_token"`
}

// EncodeCookie returns the string representation of a [Cookie]
// that should be used to store the user session in the cookies
// of a [http.ResponseWriter].
func EncodeSSOMFACookie(requestID, mfaToken string) (string, error) {
	bytes, err := json.Marshal(SSOMFACookie{RequestID: requestID, MFAToken: mfaToken})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// DecodeCookie returns the [Cookie] from the provided string.
func DecodeSSOMFACookie(b string) (*SSOMFACookie, error) {
	bytes, err := hex.DecodeString(b)
	if err != nil {
		return nil, err
	}
	var c SSOMFACookie
	if err := json.Unmarshal(bytes, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// SetMFACookie set's the SAML session cookie named by [SSOMFAToken].
func SetMFACookie(w http.ResponseWriter, requestID, token string) error {
	d, err := EncodeSSOMFACookie(requestID, token)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SSOMFAToken,
		Value:    d,
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
	})
	return nil
}

// ClearMFACookie wipes the session cookie to invalidate SAML user session.
func ClearMFACookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SSOMFAToken,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
	})
}
