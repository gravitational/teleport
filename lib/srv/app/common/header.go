/*
Copyright 2021 Gravitational, Inc.

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
	teleport.AppCFHeader,
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
