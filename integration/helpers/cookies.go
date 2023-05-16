// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
		default:
			t.Fatalf("unrecognized cookie name: %q", c.Name)
		}
	}
	return out
}
