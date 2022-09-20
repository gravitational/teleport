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

package roundtrip

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/gravitational/trace"
)

// AuthCreds hold authentication credentials for the given HTTP request
type AuthCreds struct {
	// Type is auth HTTP auth type (either Bearer or Basic)
	Type string
	// Username is HTTP username
	Username string
	// Password holds password in case of Basic auth, http token otherwize
	Password string
}

// IsToken returns whether creds are using auth token
func (a *AuthCreds) IsToken() bool {
	return a.Type == AuthBearer
}

// ParseAuthHeaders parses authentication headers from HTTP request
// it currently detects Bearer and Basic auth types
func ParseAuthHeaders(r *http.Request) (*AuthCreds, error) {
	// according to the doc below oauth 2.0 bearer access token can
	// come with query parameter
	// http://self-issued.info/docs/draft-ietf-oauth-v2-bearer.html#query-param
	// we are going to support this
	if r.URL.Query().Get(AccessTokenQueryParam) != "" {
		return &AuthCreds{
			Type:     AuthBearer,
			Password: r.URL.Query().Get(AccessTokenQueryParam),
		}, nil
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, trace.Wrap(&AccessDeniedError{Message: "unauthorized"})
	}

	auth := strings.SplitN(authHeader, " ", 2)

	if len(auth) != 2 {
		return nil, trace.Wrap(
			&ParameterError{
				Name:    "Authorization",
				Message: "invalid auth header"})
	}

	switch auth[0] {
	case AuthBasic:
		payload, err := base64.StdEncoding.DecodeString(auth[1])
		if err != nil {
			return nil, trace.Wrap(
				&ParameterError{
					Name:    "Authorization",
					Message: err.Error()})
		}
		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 {
			return nil, trace.Wrap(
				&ParameterError{
					Name:    "Authorization",
					Message: "bad header"})
		}
		return &AuthCreds{Type: AuthBasic, Username: pair[0], Password: pair[1]}, nil
	case AuthBearer:
		return &AuthCreds{Type: AuthBearer, Password: auth[1]}, nil
	}
	return nil, trace.Wrap(
		&ParameterError{
			Name:    "Authorization",
			Message: "unsupported auth scheme"})
}

const (
	// AuthBasic auth is username / password basic auth
	AuthBasic = "Basic"
	// AuthBearer auth is bearer tokens auth
	AuthBearer = "Bearer"
	// AccessTokenQueryParam URI query parameter
	AccessTokenQueryParam = "access_token"
)
