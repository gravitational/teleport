/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package oauth

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
)

// UpstreamURL returns the MCP app's upstream URL with the "mcp+" scheme
// prefix stripped, e.g. "mcp+https://mcp.linear.app/mcp" to
// "https://mcp.linear.app/mcp". Only HTTP-transport MCP apps have one.
func UpstreamURL(app types.Application) (*url.URL, error) {
	uri := app.GetURI()
	if types.GetMCPServerTransportType(uri) != types.MCPTransportHTTP {
		return nil, trace.BadParameter("MCP server %q does not use HTTP transport", app.GetName())
	}
	u, err := url.Parse(strings.TrimPrefix(uri, "mcp+"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return u, nil
}

// NewHTTPClient returns an HTTP client for MCP OAuth traffic. App-host
// well-known discovery requests are sent through the Teleport ALPN tunnel
// (as plain HTTP: the tunnel provides transport security, mirroring tsh's
// existing MCP transport). Authorization server endpoints, such as dynamic
// registration and token endpoints, are dialed directly.
//
// Host matching is by exact req.URL.Host string. That is sufficient because
// every tunneled URL is derived from the same upstream URI string the app host
// comes from.
func NewHTTPClient(getAppHost func(context.Context) (string, error), dialALPN func(context.Context) (net.Conn, error)) (*http.Client, error) {
	tunnel, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tunnel.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return dialALPN(ctx)
	}
	direct, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &http.Client{
		Transport: &routingTransport{
			getAppHost: getAppHost,
			tunnel:     tunnel,
			direct:     direct,
		},
	}, nil
}

type routingTransport struct {
	getAppHost func(context.Context) (string, error)
	tunnel     http.RoundTripper
	direct     http.RoundTripper
}

func (t *routingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	appHost, err := t.getAppHost(req.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if req.URL.Host == appHost && isWellKnownPath(req.URL.Path) {
		clone := req.Clone(req.Context())
		clone.URL.Scheme = "http"
		return t.tunnel.RoundTrip(clone)
	}
	return t.direct.RoundTrip(req)
}

func isWellKnownPath(path string) bool {
	return strings.HasPrefix(path, "/.well-known/")
}
