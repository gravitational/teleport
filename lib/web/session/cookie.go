/*
Copyright 2015 Gravitational, Inc.

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
	var c *Cookie
	if err := json.Unmarshal(bytes, &c); err != nil {
		return nil, err
	}
	return c, nil
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
