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

// GCPMiddleware implements a simplified version of MSI server serving auth tokens.
type GCPMiddleware struct {
	// Log is the Logger.
	Log logrus.FieldLogger
	// Secret to be provided by the client.
	Secret string
}

var _ LocalProxyHTTPMiddleware = &GCPMiddleware{}

func (m *GCPMiddleware) CheckAndSetDefaults() error {
	if m.Log == nil {
		m.Log = logrus.WithField(trace.Component, "gcp")
	}

	if m.Secret == "" {
		return trace.BadParameter("missing Secret")
	}
	return nil
}

func (m *GCPMiddleware) HandleRequest(rw http.ResponseWriter, req *http.Request) bool {
	auth := req.Header.Get("Authorization")
	if auth == "" {
		m.Log.Debugf("No Authorization header present, ignoring request.")
		return false
	}

	expectedAuth := "Bearer " + m.Secret

	if subtle.ConstantTimeCompare([]byte(auth), []byte(expectedAuth)) != 1 {
		m.Log.Debugf("Invalid Authorization header value %q, expected %q.", auth, expectedAuth)
		trace.WriteError(rw, trace.BadParameter("Invalid Authorization header"))
		return true
	}

	return false
}
