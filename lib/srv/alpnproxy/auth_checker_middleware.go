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

package alpnproxy

import (
	"crypto/subtle"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// AuthorizationCheckerMiddleware is a middleware that checks `Authorization` header of incoming requests.
// If the header is missing, the request is passed through.
// If it is present, the middleware checks it is a bearer token with value matching the secret.
type AuthorizationCheckerMiddleware struct {
	// Log is the Logger.
	Log logrus.FieldLogger
	// Secret is the expected value of a bearer token.
	Secret string
}

var _ LocalProxyHTTPMiddleware = (*AuthorizationCheckerMiddleware)(nil)

// CheckAndSetDefaults checks configuration validity and sets defaults.
func (m *AuthorizationCheckerMiddleware) CheckAndSetDefaults() error {
	if m.Log == nil {
		m.Log = logrus.WithField(trace.Component, "gcp")
	}

	if m.Secret == "" {
		return trace.BadParameter("missing Secret")
	}
	return nil
}

// HandleRequest checks Authorization header, which must be either missing or set to the secret value of a bearer token.
func (m *AuthorizationCheckerMiddleware) HandleRequest(rw http.ResponseWriter, req *http.Request) bool {
	auth := req.Header.Get("Authorization")
	if auth == "" {
		m.Log.Debugf("No Authorization header present, ignoring request.")
		return false
	}

	expectedAuth := "Bearer " + m.Secret

	if subtle.ConstantTimeCompare([]byte(auth), []byte(expectedAuth)) != 1 {
		trace.WriteError(rw, trace.AccessDenied("Invalid Authorization header"))
		return true
	}

	return false
}

func (m *AuthorizationCheckerMiddleware) HandleResponse(*http.Response) error {
	return nil
}
