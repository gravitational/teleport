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

	// ClearCerts clears the middleware certs.
	// It will try to reissue them when a new request comes in.
	ClearCerts()
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
func (m *DefaultLocalProxyHTTPMiddleware) ClearCerts() {}
