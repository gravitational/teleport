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

package servicecfg

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/net/http/httpguts"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/gravitational/teleport/api/types"
	netutils "github.com/gravitational/teleport/api/utils/net"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/app/common"
)

// AppsConfig configures application proxy service.
type AppsConfig struct {
	// Enabled enables application proxying service.
	Enabled bool

	// DebugApp enabled a header dumping debugging application.
	DebugApp bool

	// Apps is the list of applications that are being proxied.
	Apps []App

	// ResourceMatchers match cluster database resources.
	ResourceMatchers []services.ResourceMatcher

	// MonitorCloseChannel will be signaled when a monitor closes a connection.
	// Used only for testing. Optional.
	MonitorCloseChannel chan struct{}
}

// App is the specific application that will be proxied by the application
// service. This needs to exist because if the "config" package tries to
// directly create a services.App it will get into circular imports.
type App struct {
	// Name of the application.
	Name string

	// Description is the app description.
	Description string

	// URI is the internal address of the application.
	URI string

	// Public address of the application. This is the address users will access
	// the application at.
	PublicAddr string

	// StaticLabels is a map of static labels to apply to this application.
	StaticLabels map[string]string

	// DynamicLabels is a list of dynamic labels to apply to this application.
	DynamicLabels services.CommandLabels

	// InsecureSkipVerify is used to skip validating the server's certificate.
	InsecureSkipVerify bool

	// Rewrite defines a block that is used to rewrite requests and responses.
	Rewrite *Rewrite

	// AWS contains additional options for AWS applications.
	AWS *AppAWS

	// Cloud identifies the cloud instance the app represents.
	Cloud string

	// RequiredAppNames is a list of app names that are required for this app to function. Any app listed here will
	// be part of the authentication redirect flow and authenticate along side this app.
	RequiredAppNames []string

	// UseAnyProxyPublicAddr will rebuild this app's fqdn based on the proxy public addr that the
	// request originated from. This should be true if your proxy has multiple proxy public addrs and you
	// want the app to be accessible from any of them. If `public_addr` is explicitly set in the app spec,
	// setting this value to true will overwrite that public address in the web UI.
	UseAnyProxyPublicAddr bool

	// CORS defines the Cross-Origin Resource Sharing configuration for the app,
	// controlling how resources are shared across different origins.
	CORS *CORS

	// TCPPorts is a list of ports and port ranges that an app agent can forward connections to.
	// Only applicable to TCP App Access.
	// If this field is not empty, URI is expected to contain no port number and start with the tcp
	// protocol.
	TCPPorts []PortRange
}

// CORS represents the configuration for Cross-Origin Resource Sharing (CORS)
// settings that control how the app responds to requests from different origins.
type CORS struct {
	// AllowedOrigins specifies the list of origins that are allowed to access the app.
	// Example: "https://client.teleport.example.com:3080"
	AllowedOrigins []string `yaml:"allowed_origins"`

	// AllowedMethods specifies the HTTP methods that are allowed when accessing the app.
	// Example: "POST", "GET", "OPTIONS", "PUT", "DELETE"
	AllowedMethods []string `yaml:"allowed_methods"`

	// AllowedHeaders specifies the HTTP headers that can be used when making requests to the app.
	// Example: "Content-Type", "Authorization", "X-Custom-Header"
	AllowedHeaders []string `yaml:"allowed_headers"`

	// ExposedHeaders indicate which response headers should be made available to scripts running in
	// the browser, in response to a cross-origin request.
	ExposedHeaders []string `yaml:"exposed_headers"`

	// AllowCredentials indicates whether credentials such as cookies or authorization headers
	// are allowed to be included in the requests.
	AllowCredentials bool `yaml:"allow_credentials"`

	// MaxAge specifies how long (in seconds) the results of a preflight request can be cached.
	// Example: 86400 (which equals 24 hours)
	MaxAge uint `yaml:"max_age"`
}

// PortRange describes a port range for TCP apps. The range starts with Port and ends with EndPort.
// PortRange can be used to describe a single port in which case the Port field is the port and the
// EndPort field is 0.
type PortRange struct {
	// Port describes the start of the range. It must be between 1 and 65535.
	Port int
	// EndPort describes the end of the range, inclusive. When describing a port range, it must be
	// greater than Port and less than or equal to 65535. When describing a single port, it must be
	// set to 0.
	EndPort int
}

// CheckAndSetDefaults validates an application.
func (a *App) CheckAndSetDefaults() error {
	if a.Name == "" {
		return trace.BadParameter("missing application name")
	}
	if a.URI == "" {
		if a.Cloud != "" {
			a.URI = fmt.Sprintf("cloud://%v", a.Cloud)
		} else {
			return trace.BadParameter("missing application %q URI", a.Name)
		}
	}
	// Check if the application name is a valid subdomain. Don't allow names that
	// are invalid subdomains because for trusted clusters the name is used to
	// construct the domain that the application will be available at.
	if errs := validation.IsDNS1035Label(a.Name); len(errs) > 0 {
		return trace.BadParameter("application name %q must be a valid DNS subdomain: https://goteleport.com/docs/enroll-resources/application-access/guides/connecting-apps/#application-name", a.Name)
	}
	// Parse and validate URL.
	if _, err := url.Parse(a.URI); err != nil {
		return trace.BadParameter("application %q URI invalid: %v", a.Name, err)
	}
	// If a port was specified or an IP address was provided for the public
	// address, return an error.
	if a.PublicAddr != "" {
		if _, _, err := net.SplitHostPort(a.PublicAddr); err == nil {
			return trace.BadParameter("application %q public_addr %q can not contain a port, applications will be available on the same port as the web proxy", a.Name, a.PublicAddr)
		}
		if net.ParseIP(a.PublicAddr) != nil {
			return trace.BadParameter("application %q public_addr %q can not be an IP address, Teleport Application Access uses DNS names for routing", a.Name, a.PublicAddr)
		}
	}
	// Mark the app as coming from the static configuration.
	if a.StaticLabels == nil {
		a.StaticLabels = make(map[string]string)
	}
	a.StaticLabels[types.OriginLabel] = types.OriginConfigFile
	// Make sure there are no reserved headers in the rewrite configuration.
	// They wouldn't be rewritten even if we allowed them here but catch it
	// early and let the user know.
	if a.Rewrite != nil {
		for _, h := range a.Rewrite.Headers {
			if common.IsReservedHeader(h.Name) {
				return trace.BadParameter("invalid application %q header rewrite configuration: header %q is reserved and can't be rewritten",
					a.Name, http.CanonicalHeaderKey(h.Name))
			}
		}
	}

	if len(a.TCPPorts) != 0 {
		if err := a.checkPorts(); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (a *App) checkPorts() error {
	// Parsing the URI here does not break compatibility. The URI is parsed only if Ports are present.
	// This means that old apps that do have invalid URIs but don't use Ports can continue existing.
	uri, err := url.Parse(a.URI)
	if err != nil {
		return trace.BadParameter("invalid app URI format: %v", err)
	}

	// The scheme of URI is not validated to be "tcp" on purpose. This way in the future we can add
	// multi-port support to web apps without throwing hard errors when a cluster with a multi-port
	// web app gets downgraded to a version which supports multi-port only for TCP apps.
	//
	// For now, we simply ignore the Ports field set on non-TCP apps.
	if uri.Scheme != "tcp" {
		return nil
	}

	if uri.Port() != "" {
		return trace.BadParameter("app URI %q must not include a port number when the app spec defines a list of ports", a.URI)
	}

	for _, portRange := range a.TCPPorts {
		if err := netutils.ValidatePortRange(portRange.Port, portRange.EndPort); err != nil {
			return trace.Wrap(err, "validating a port range of a TCP app")
		}
	}

	return nil
}

// AppAWS contains additional options for AWS applications.
type AppAWS struct {
	// ExternalID is the AWS External ID used when assuming roles in this app.
	ExternalID string
}

// Rewrite is a list of rewriting rules to apply to requests and responses.
type Rewrite struct {
	// Redirect is a list of hosts that should be rewritten to the public address.
	Redirect []string
	// Headers is a list of extra headers to inject in the request.
	Headers []Header
	// JWTClaims configures whether roles/traits are included in the JWT token.
	JWTClaims string
}

// Header represents a single http header passed over to the proxied application.
type Header struct {
	// Name is the http header name.
	Name string
	// Value is the http header value.
	Value string
}

// ParseHeader parses the provided string as a http header.
func ParseHeader(header string) (*Header, error) {
	parts := strings.SplitN(header, ":", 2)
	if len(parts) != 2 {
		return nil, trace.BadParameter("failed to parse %q as http header", header)
	}
	name := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if !httpguts.ValidHeaderFieldName(name) {
		return nil, trace.BadParameter("invalid http header name: %q", header)
	}
	if !httpguts.ValidHeaderFieldValue(value) {
		return nil, trace.BadParameter("invalid http header value: %q", header)
	}
	return &Header{
		Name:  name,
		Value: value,
	}, nil
}

// ParseHeaders parses the provided list as http headers.
func ParseHeaders(headers []string) (headersOut []Header, err error) {
	for _, header := range headers {
		h, err := ParseHeader(header)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		headersOut = append(headersOut, *h)
	}
	return headersOut, nil
}
