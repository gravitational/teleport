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
	"net/http"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/httplib/reverseproxy"
)

// SetTeleportAPIErrorHeader saves the provided error in X-Teleport-API-Error header of response.
func SetTeleportAPIErrorHeader(rw http.ResponseWriter, err error) {
	obj, ok := err.(trace.DebugReporter)
	if !ok {
		obj = &trace.TraceErr{Err: err}
	}
	rw.Header().Set(TeleportAPIErrorHeader, obj.DebugReport())
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
