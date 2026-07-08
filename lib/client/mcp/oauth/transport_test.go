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
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestUpstreamURL(t *testing.T) {
	app, err := types.NewAppV3(types.Metadata{Name: "linear"}, types.AppSpecV3{
		URI: "mcp+https://mcp.linear.app/mcp",
	})
	require.NoError(t, err)
	u, err := UpstreamURL(app)
	require.NoError(t, err)
	require.Equal(t, "https://mcp.linear.app/mcp", u.String())

	stdioApp, err := types.NewAppV3(types.Metadata{Name: "everything"}, types.AppSpecV3{
		MCP: &types.MCP{Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-everything"}, RunAsHostUser: "user"},
	})
	require.NoError(t, err)
	_, err = UpstreamURL(stdioApp)
	require.Error(t, err)
}

func TestNewHTTPClientRouting(t *testing.T) {
	// "App" server, reachable only through the fake tunnel dialer.
	appSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "app:"+r.URL.Path)
	}))
	defer appSrv.Close()
	// "Authorization server", reachable only directly.
	asSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "as:"+r.URL.Path)
	}))
	defer asSrv.Close()

	dialALPN := func(ctx context.Context) (net.Conn, error) {
		return net.Dial("tcp", appSrv.Listener.Addr().String())
	}
	// The app's advertised host is a name that does NOT resolve: reaching it
	// proves the request went through the tunnel dialer.
	client, err := NewHTTPClient(func(context.Context) (string, error) {
		return "upstream.invalid:9204", nil
	}, dialALPN)
	require.NoError(t, err)

	// App-host well-known discovery request (note: https scheme must be
	// rewritten to plain HTTP over the tunnel, mirroring tsh's existing
	// behavior).
	resp, err := client.Get("https://upstream.invalid:9204/.well-known/oauth-protected-resource/mcp")
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, "app:/.well-known/oauth-protected-resource/mcp", string(body))

	// Non-app-host request goes direct.
	resp, err = client.Get(asSrv.URL + "/token")
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, "as:/token", string(body))
}
