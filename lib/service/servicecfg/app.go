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
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/net/http/httpguts"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/app"
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

	var headerNames []string
	if a.Rewrite != nil {
		for _, h := range a.Rewrite.Headers {
			headerNames = append(headerNames, h.Name)
		}
	}
	if err := app.ValidateApplication(a.Name, a.URI, a.PublicAddr, headerNames); err != nil {
		return trace.Wrap(err)
	}

	// Mark the app as coming from the static configuration.
	if a.StaticLabels == nil {
		a.StaticLabels = make(map[string]string)
	}
	a.StaticLabels[types.OriginLabel] = types.OriginConfigFile

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
