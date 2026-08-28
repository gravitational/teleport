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
)

// LocalProxyHTTPMiddleware provides callback functions for LocalProxy in HTTP proxy mode.
type LocalProxyHTTPMiddleware interface {
	// CheckAndSetDefaults checks configuration validity and sets defaults
	CheckAndSetDefaults() error

	// HandleRequest returns true if requests has been handled and must not be processed further, false otherwise.
	HandleRequest(rw http.ResponseWriter, req *http.Request) bool

	// HandleResponse processes the server response before sending it to the client.
	HandleResponse(resp *http.Response) error

	// GetClientCerts optionally returns client certs that should be used for
	// the connection to the upstream server for the given request. The method
	// can return a false bool to signal that the [LocalProxy] should use the
	// client cert from its own config.
	GetClientCerts(req *http.Request) ([]tls.Certificate, bool, error)

	// OverwriteServerName optionally returns a SNI that should be used for the
	// connection to the upstream server for the given request, or a false bool
	// to use the SNI from the [LocalProxy]'s own config.
	GetServerName(req *http.Request) (string, bool, error)

	// ClearCerts clears the middleware certs.
	// It will try to reissue them when a new request comes in.
	ClearCerts()
}

// DefaultLocalProxyHTTPMiddleware provides default no-op implementations for
// [LocalProxyHTTPMiddleware].
type DefaultLocalProxyHTTPMiddleware struct {
}

// CheckAndSetDefaults implements [LocalProxyHTTPMiddleware].
func (DefaultLocalProxyHTTPMiddleware) CheckAndSetDefaults() error {
	return nil
}

// HandleRequest implements [LocalProxyHTTPMiddleware].
func (DefaultLocalProxyHTTPMiddleware) HandleRequest(rw http.ResponseWriter, req *http.Request) bool {
	return false
}

// HandleResponse implements [LocalProxyHTTPMiddleware].
func (DefaultLocalProxyHTTPMiddleware) HandleResponse(resp *http.Response) error {
	return nil
}

// GetClientCerts implements [LocalProxyHTTPMiddleware].
func (DefaultLocalProxyHTTPMiddleware) GetClientCerts(req *http.Request) ([]tls.Certificate, bool, error) {
	return nil, false, nil
}

// GetServerName implements [LocalProxyHTTPMiddleware].
func (DefaultLocalProxyHTTPMiddleware) GetServerName(req *http.Request) (string, bool, error) {
	return "", false, nil
}

// ClearCerts implements [LocalProxyHTTPMiddleware].
func (DefaultLocalProxyHTTPMiddleware) ClearCerts() {}
