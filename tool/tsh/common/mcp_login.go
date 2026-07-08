/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/sso"
	"github.com/gravitational/teleport/lib/defaults"
)

type mcpLoginCommand struct {
	*kingpin.CmdClause
	cf *CLIConf
}

func newMCPLoginCommand(parent *kingpin.CmdClause, cf *CLIConf) *mcpLoginCommand {
	cmd := &mcpLoginCommand{
		CmdClause: parent.Command("login", "Log in to an OAuth-protected MCP server."),
		cf:        cf,
	}
	cmd.Arg("name", "Name of the MCP server.").Required().StringVar(&cf.AppName)
	return cmd
}

func (c *mcpLoginCommand) run() error {
	ctx := c.cf.Context
	tc, err := makeClient(c.cf)
	if err != nil {
		return trace.Wrap(err)
	}

	dialer := client.NewMCPServerDialer(tc, c.cf.AppName)
	app, err := dialer.GetApp(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if types.GetMCPServerTransportType(app.GetURI()) != types.MCPTransportHTTP {
		return trace.BadParameter("MCP server %q does not use HTTP transport; OAuth login only applies to HTTP MCP servers", c.cf.AppName)
	}

	httpClient, err := newMCPOAuthHTTPClient(dialer)
	if err != nil {
		return trace.Wrap(err)
	}

	// The loopback listener that catches the browser redirect. It must exist
	// before dynamic client registration, since the exact redirect URI (port
	// included) is part of what gets registered.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return trace.Wrap(err)
	}
	defer listener.Close()
	redirectURI := fmt.Sprintf("http://%s/callback", listener.Addr())

	tokenStore := mcpclienttransport.NewMemoryTokenStore()
	oauthHandler := mcpclienttransport.NewOAuthHandler(mcpclienttransport.OAuthConfig{
		RedirectURI: redirectURI,
		PKCEEnabled: true,
		HTTPClient:  httpClient,
		TokenStore:  tokenStore,
	})
	oauthHandler.SetBaseURL("http://localhost")

	fmt.Fprintf(c.cf.Stdout(), "Registering OAuth client for MCP server %q...\n", c.cf.AppName)
	if err := oauthHandler.RegisterClient(ctx, "Teleport tsh"); err != nil {
		return trace.Wrap(err)
	}

	codeVerifier, err := mcpclienttransport.GenerateCodeVerifier()
	if err != nil {
		return trace.Wrap(err)
	}
	state, err := mcpclienttransport.GenerateState()
	if err != nil {
		return trace.Wrap(err)
	}
	authURL, err := oauthHandler.GetAuthorizationURL(ctx, state, mcpclienttransport.GenerateCodeChallenge(codeVerifier))
	if err != nil {
		return trace.Wrap(err)
	}

	callbackCh := make(chan url.Values, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		select {
		case callbackCh <- r.URL.Query():
		default: // Duplicate callback, first one wins.
		}
		fmt.Fprintln(w, "Login complete. You can close this tab and return to the terminal.")
	})
	callbackServer := &http.Server{Handler: mux}
	go callbackServer.Serve(listener)
	defer callbackServer.Close()

	fmt.Fprintf(c.cf.Stdout(), "Opening browser for authorization. If it does not open, visit:\n\n  %v\n\n", authURL)
	if err := sso.OpenURLInBrowser(c.cf.Browser, authURL); err != nil {
		fmt.Fprintf(c.cf.Stderr(), "Failed to open a browser: %v\n", err)
	}

	var query url.Values
	select {
	case query = <-callbackCh:
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	case <-time.After(3 * time.Minute):
		return trace.LimitExceeded("timed out waiting for the browser authorization to complete")
	}
	if errCode := query.Get("error"); errCode != "" {
		return trace.AccessDenied("authorization failed: %v: %v", errCode, query.Get("error_description"))
	}

	if err := oauthHandler.ProcessAuthorizationResponse(ctx, query.Get("code"), query.Get("state"), codeVerifier); err != nil {
		return trace.Wrap(err)
	}

	token, err := tokenStore.GetToken(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	credsPath := mcpOAuthTokenPath(c.cf.HomePath, tc.WebProxyHost(), tc.SiteName, c.cf.AppName)
	if err := saveMCPOAuthCredentials(credsPath, &mcpOAuthCredentials{
		ClientID: oauthHandler.GetClientID(),
		Token:    *token,
	}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintf(c.cf.Stdout(), "Authorization complete. Tokens stored in %v.\n", credsPath)
	fmt.Fprintf(c.cf.Stdout(), "MCP server %q is ready — restart your MCP clients if already running.\n", c.cf.AppName)
	return nil
}

// newMCPOAuthHTTPClient returns the HTTP client for talking OAuth. The
// ceremony talks to two different places: the MCP server (for discovery),
// which may only be reachable through the Teleport proxy, and the
// authorization server (for registration, token exchange, and refresh),
// which must be directly reachable since the browser goes there anyway.
// Requests to "localhost" are the MCP server via the tunnel; everything
// else goes out directly.
func newMCPOAuthHTTPClient(dialer *client.MCPServerDialer) (*http.Client, error) {
	tunneled, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tunneled.DialContext = dialer.DialContext
	direct, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &http.Client{
		Transport: &hostRoutingTransport{tunneled: tunneled, direct: direct},
		Timeout:   30 * time.Second,
	}, nil
}

// hostRoutingTransport sends requests addressed to "localhost" through the
// Teleport ALPN tunnel to the MCP server and everything else (the
// authorization server) directly.
type hostRoutingTransport struct {
	tunneled http.RoundTripper
	direct   http.RoundTripper
}

func (t *hostRoutingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Hostname() == "localhost" {
		return t.tunneled.RoundTrip(r)
	}
	return t.direct.RoundTrip(r)
}
