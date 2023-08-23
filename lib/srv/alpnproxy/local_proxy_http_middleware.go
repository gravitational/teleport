/*
Copyright 2023 Gravitational, Inc.

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

package alpnproxy

import (
	"crypto/tls"
	"net/http"

	"github.com/gravitational/trace"
)

// LocalProxyHTTPMiddleware provides callback functions for LocalProxy in HTTP proxy mode.
type LocalProxyHTTPMiddleware interface {
	// CheckAndSetDefaults checks configuration validity and sets defaults
	CheckAndSetDefaults() error

	// HandleRequest returns true if requests has been handled and must not be processed further, false otherwise.
	HandleRequest(rw http.ResponseWriter, req *http.Request) bool

	// HandleResponse processes the server response before sending it to the client.
	HandleResponse(resp *http.Response) error

	// OverwriteClientCerts overwrites the client certs used for upstream connection.
	OverwriteClientCerts(req *http.Request) ([]tls.Certificate, error)
}

// DefaultLocalProxyHTTPMiddleware provides default implementations for LocalProxyHTTPMiddleware.
type DefaultLocalProxyHTTPMiddleware struct {
}

func (m *DefaultLocalProxyHTTPMiddleware) CheckAndSetDefaults() error {
	return nil
}
func (m *DefaultLocalProxyHTTPMiddleware) HandleRequest(rw http.ResponseWriter, req *http.Request) bool {
	return false
}
func (m *DefaultLocalProxyHTTPMiddleware) HandleResponse(resp *http.Response) error {
	return nil
}
func (m *DefaultLocalProxyHTTPMiddleware) OverwriteClientCerts(req *http.Request) ([]tls.Certificate, error) {
	return nil, trace.NotImplemented("not implemented")
}
