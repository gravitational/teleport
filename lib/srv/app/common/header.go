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

package common

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/httplib/reverseproxy"
)

// SetTeleportAPIErrorHeader saves the provided error in X-Teleport-API-Error header of response.
func SetTeleportAPIErrorHeader(rw http.ResponseWriter, err error) {
	var debugErr trace.DebugReporter
	if !errors.As(err, &debugErr) {
		debugErr = &trace.TraceErr{Err: err}
	}
	rw.Header().Set(TeleportAPIErrorHeader, debugErr.DebugReport())
}

const (
	// XForwardedSSL is a non-standard X-Forwarded-* header that is set to "on" or "off" depending on
	// whether SSL is enabled.
	XForwardedSSL = "X-Forwarded-Ssl"

	// TeleportAPIErrorHeader is Teleport-specific error header, optionally holding background error information.
	TeleportAPIErrorHeader = "X-Teleport-Api-Error"

	// TeleportAPIInfoHeader is Teleport-specific info header, optionally holding background information.
	TeleportAPIInfoHeader = "X-Teleport-Api-Info"

	// TeleportAWSAssumedRole indicates that the incoming requests are signed
	// with real AWS credentials of the specified assumed role by the AWS client.
	TeleportAWSAssumedRole = "X-Teleport-Aws-Assumed-Role"

	// TeleportAWSAssumedRoleAuthorization contains the original authorization
	// header for requests signed by assumed roles.
	TeleportAWSAssumedRoleAuthorization = "X-Teleport-Aws-Assumed-Role-Authorization"

	// TeleportAppErrorCodeHeader holds an [AppErrorCode] when Teleport denies a
	// forwarded client identity access to an application, or app_resources
	// rules deny a request on either path. Teleport removes it from upstream
	// responses, so that a client can trust it.
	TeleportAppErrorCodeHeader = "X-Teleport-App-Error-Code"
)

// reservedResponseHeaders are the response headers that only Teleport may set,
// so that clients can trust them.
var reservedResponseHeaders = []string{TeleportAppErrorCodeHeader}

// RemoveReservedResponseHeaders removes the headers that only Teleport may set
// from h, whether as headers or trailers, so that an upstream can't forge them.
func RemoveReservedResponseHeaders(h http.Header) {
	for key := range h {
		// Trailer keys set after the body are prefixed and not canonicalized.
		name := http.CanonicalHeaderKey(strings.TrimPrefix(key, http.TrailerPrefix))
		if slices.Contains(reservedResponseHeaders, name) {
			delete(h, key)
		}
	}
}

// WithoutReservedResponseHeaders serves next, removing the headers that only
// Teleport may set from everything next writes: interim (1xx) responses, the
// final response and its trailers. A protocol upgrade's 101 response is
// written to the hijacked connection rather than through the writer, so a
// handler that proxies upgrades must also call RemoveReservedResponseHeaders
// on the upstream's response.
func WithoutReservedResponseHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(reservedHeaderWriter{w}, r)
		// The server sends trailers once this returns.
		RemoveReservedResponseHeaders(w.Header())
	})
}

// reservedHeaderWriter removes reserved headers before every call that may
// send a header block.
type reservedHeaderWriter struct{ http.ResponseWriter }

func (w reservedHeaderWriter) WriteHeader(code int) {
	RemoveReservedResponseHeaders(w.Header())
	w.ResponseWriter.WriteHeader(code)
}

func (w reservedHeaderWriter) Write(b []byte) (int, error) {
	RemoveReservedResponseHeaders(w.Header())
	return w.ResponseWriter.Write(b)
}

func (w reservedHeaderWriter) Flush() {
	RemoveReservedResponseHeaders(w.Header())
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

// Unwrap lets http.ResponseController reach the server's writer, which
// ReverseProxy uses to hijack upgrades.
func (w reservedHeaderWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// AppErrorCode is a stable code that tells a client why Teleport rejected a
// request. The response body is guidance for a person, so clients should rely
// on the code rather than the body's wording.
type AppErrorCode string

const (
	// AppErrorCodeAccessDenied is an authorization failure without a more specific code. It is also the code when the
	// client may not discover the application, so that the response reveals nothing about it.
	AppErrorCodeAccessDenied AppErrorCode = "access-denied"
	// AppErrorCodeMFARequired means that access requires per-session MFA, which a forwarded client identity can't
	// satisfy.
	AppErrorCodeMFARequired AppErrorCode = "mfa-required"
	// AppErrorCodeUnsupportedApplicationType means that the application can't be served with a forwarded client
	// identity and needs an application-specific access path.
	AppErrorCodeUnsupportedApplicationType AppErrorCode = "unsupported-application-type"
	// AppErrorCodeResourceDenied means that the application's app_resources
	// rules deny the request path or method. The client may still access other
	// paths of the application.
	AppErrorCodeResourceDenied AppErrorCode = "resource-denied"
	// AppErrorCodeSessionInvalid means the app session is invalid, e.g. due to
	// a backing role or user resource being deleted.
	AppErrorCodeSessionInvalid AppErrorCode = "session-invalid"
)

// ReservedHeaders is a list of headers injected by Teleport.
var ReservedHeaders = append([]string{
	teleport.AppJWTHeader,
	XForwardedSSL,
	TeleportAPIErrorHeader,
	TeleportAPIInfoHeader,
	TeleportAWSAssumedRole,
	TeleportAWSAssumedRoleAuthorization,
},
	reverseproxy.XHeaders...,
)

var requestOverrideHeaders = []string{
	"X-Http-Method",
	"X-Http-Method-Override",
	"X-Method-Override",
	"X-Original-Uri",
	"X-Original-Url",
	"X-Rewrite-Url",
}

func normalizeHeaderName(name string) string {
	return http.CanonicalHeaderKey(strings.ReplaceAll(name, "_", "-"))
}

// IsRequestOverrideHeader returns true when an upstream app may act on header
// as the request method or the request path.
func IsRequestOverrideHeader(header string) bool {
	return slices.Contains(requestOverrideHeaders, normalizeHeaderName(header))
}

// IsReservedHeader returns true if the provided header is one of headers
// injected by Teleport.
func IsReservedHeader(header string) bool {
	for _, h := range ReservedHeaders {
		if http.CanonicalHeaderKey(header) == http.CanonicalHeaderKey(h) {
			return true
		}
	}
	return false
}
