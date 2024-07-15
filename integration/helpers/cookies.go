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

package helpers

import (
	"net/http"
	"testing"

	"github.com/gravitational/teleport/lib/web/app"
)

// AppCookies is a helper struct containing application session cookies parsed from a slice of cookies.
type AppCookies struct {
	SessionCookie        *http.Cookie
	SubjectSessionCookie *http.Cookie
	AuthStateCookie      *http.Cookie
}

// WithSubjectCookie returns a copy of AppCookies with the specified subject session cookie.
func (ac *AppCookies) WithSubjectCookie(c *http.Cookie) *AppCookies {
	copy := *ac
	copy.SubjectSessionCookie = c
	return &copy
}

// ToSlice is a convenience method for converting non-nil AppCookes into a slice of cookies.
func (ac *AppCookies) ToSlice() []*http.Cookie {
	var out []*http.Cookie
	if ac.SessionCookie != nil {
		out = append(out, ac.SessionCookie)
	}
	if ac.SubjectSessionCookie != nil {
		out = append(out, ac.SubjectSessionCookie)
	}
	if ac.AuthStateCookie != nil {
		out = append(out, ac.AuthStateCookie)
	}
	return out
}

// ParseCookies parses a slice of application session cookies into an AppCookies struct.
func ParseCookies(t *testing.T, cookies []*http.Cookie) *AppCookies {
	t.Helper()
	out := &AppCookies{}
	for _, c := range cookies {
		switch c.Name {
		case app.CookieName:
			out.SessionCookie = c
		case app.SubjectCookieName:
			out.SubjectSessionCookie = c
		case app.AuthStateCookieName:
			out.AuthStateCookie = c
		default:
			t.Fatalf("unrecognized cookie name: %q", c.Name)
		}
	}
	return out
}
