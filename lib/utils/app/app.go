// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package app

import (
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/httplib/reverseproxy"
)

// AssembleAppFQDN returns the application's FQDN.
//
// If the application is running within the local cluster and it has a public
// address specified, the application's public address is used.
//
// In all other cases, i.e. if the public address is not set or the application
// is running in a remote cluster, the FQDN is formatted as
// <appName>.<localProxyDNSName>
func AssembleAppFQDN(localClusterName string, localProxyDNSName string, appClusterName string, app types.Application) string {
	isLocalCluster := localClusterName == appClusterName
	if isLocalCluster && app.GetPublicAddr() != "" {
		return app.GetPublicAddr()
	}
	return DefaultAppPublicAddr(app.GetName(), localProxyDNSName)
}

// DefaultAppPublicAddr returns the default publicAddr for an app.
// Format: <appName>.<localProxyDNSName>
func DefaultAppPublicAddr(appName, localProxyDNSName string) string {
	return fmt.Sprintf("%v.%v", appName, localProxyDNSName)
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

// ValidateApplication checks the name, URI, public address and headers
// of an Application.
func ValidateApplication(name, uri, publicAddr string, headerNames []string) error {
	// Check if the application name is a valid subdomain. Don't allow names that
	// are invalid subdomains because for trusted clusters the name is used to
	// construct the domain that the application will be available at.
	if errs := validation.IsDNS1035Label(name); len(errs) > 0 {
		return trace.BadParameter("application name %q must be a valid DNS subdomain: https://goteleport.com/docs/application-access/guides/connecting-apps/#application-name", name)
	}
	// Parse and validate URL.
	if _, err := url.Parse(uri); err != nil {
		return trace.BadParameter("application %q URI invalid: %v", name, err)
	}
	// If a port was specified or an IP address was provided for the public
	// address, return an error.
	if publicAddr != "" {
		if _, _, err := net.SplitHostPort(publicAddr); err == nil {
			return trace.BadParameter("application %q public_addr %q can not contain a port, applications will be available on the same port as the web proxy", name, publicAddr)
		}
		if net.ParseIP(publicAddr) != nil {
			return trace.BadParameter("application %q public_addr %q can not be an IP address, Teleport Application Access uses DNS names for routing", name, publicAddr)
		}
	}
	// Make sure there are no reserved headers in the rewrite configuration.
	// They wouldn't be rewritten even if we allowed them here but catch it
	// early and let the user know.
	for _, header := range headerNames {
		if IsReservedHeader(header) {
			return trace.BadParameter("invalid application %q header rewrite configuration: header %q is reserved and can't be rewritten",
				name, http.CanonicalHeaderKey(header))
		}
	}

	return nil
}
