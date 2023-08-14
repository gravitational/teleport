// Copyright 2023 Gravitational, Inc
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
		return trace.BadParameter("application name %q must be a valid DNS subdomain: https://goteleport.com/docs/application-access/guides/connecting-apps/#application-name", a.Name)
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
