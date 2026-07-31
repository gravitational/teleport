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
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/sso"
	"github.com/gravitational/teleport/lib/defaults"
)

type mcpLoginCommand struct {
	*kingpin.CmdClause
	cf           *CLIConf
	clientID     string
	promptSecret bool
	callbackPort uint16
}

func newMCPLoginCommand(parent *kingpin.CmdClause, cf *CLIConf) *mcpLoginCommand {
	cmd := &mcpLoginCommand{
		CmdClause: parent.Command("login", "Log in to an OAuth-protected MCP server."),
		cf:        cf,
	}
	cmd.Arg("name", "Name of the MCP server.").Required().SetValue(&cf.AppSQN)
	cmd.Flag("client-id", "OAuth client ID for a pre-registered client. When set, dynamic client registration is skipped.").
		StringVar(&cmd.clientID)
	cmd.Flag("client-secret", "Prompt for the OAuth client secret of a pre-registered confidential client.").
		BoolVar(&cmd.promptSecret)
	cmd.Flag("callback-port", "Local OAuth callback port. Set this to the exact port registered with the OAuth provider.").
		Uint16Var(&cmd.callbackPort)
	return cmd
}

func (c *mcpLoginCommand) run() error {
	ctx := c.cf.Context
	tc, err := makeClient(c.cf)
	if err != nil {
		return trace.Wrap(err)
	}

	dialer := client.NewMCPServerDialer(tc, c.cf.AppSQN.Name)
	app, err := dialer.GetApp(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if types.GetMCPServerTransportType(app.GetURI()) != types.MCPTransportHTTP {
		return trace.BadParameter("MCP server %q does not use HTTP transport; OAuth login only applies to HTTP MCP servers", c.cf.AppSQN.Name)
	}

	clientID, clientSecret, err := c.getOAuthClientCredentials()
	if err != nil {
		return trace.Wrap(err)
	}

	httpClient, err := newMCPOAuthHTTPClient(dialer)
	if err != nil {
		return trace.Wrap(err)
	}

	// The loopback listener that catches the browser redirect. It must exist
	// before dynamic client registration, since the exact redirect URI (port
	// included) is part of what gets registered.
	listenAddr := "127.0.0.1:0"
	if c.callbackPort != 0 {
		listenAddr = fmt.Sprintf("localhost:%d", c.callbackPort)
	}
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return trace.Wrap(err)
	}
	defer listener.Close()
	redirectURI := fmt.Sprintf("http://%s/callback", listener.Addr())
	if c.callbackPort != 0 {
		redirectURI = fmt.Sprintf("http://localhost:%d/callback", c.callbackPort)
	}

	tokenStore := mcpclienttransport.NewMemoryTokenStore()
	oauthHandler := mcpclienttransport.NewOAuthHandler(mcpclienttransport.OAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		PKCEEnabled:  true,
		HTTPClient:   httpClient,
		TokenStore:   tokenStore,
	})
	oauthHandler.SetBaseURL("http://localhost")

	if clientID == "" {
		fmt.Fprintf(c.cf.Stdout(), "Registering OAuth client for MCP server %q...\n", c.cf.AppSQN.Name)
		if err := oauthHandler.RegisterClient(ctx, "Teleport tsh"); err != nil {
			return trace.Wrap(err)
		}
	} else {
		fmt.Fprintf(c.cf.Stdout(), "Using pre-registered OAuth client for MCP server %q...\n", c.cf.AppSQN.Name)
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
	credsPath := mcpOAuthTokenPath(c.cf.HomePath, tc.WebProxyHost(), tc.SiteName, c.cf.AppSQN.Name)
	if err := saveMCPOAuthCredentials(credsPath, &mcpOAuthCredentials{
		ClientID:     oauthHandler.GetClientID(),
		ClientSecret: oauthHandler.GetClientSecret(),
		Token:        *token,
	}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintf(c.cf.Stdout(), "Authorization complete. Tokens stored in %v.\n", credsPath)
	fmt.Fprintf(c.cf.Stdout(), "MCP server %q is ready — restart your MCP clients if already running.\n", c.cf.AppSQN.Name)
	return nil
}

func (c *mcpLoginCommand) getOAuthClientCredentials() (string, string, error) {
	clientID := strings.TrimSpace(c.clientID)
	if !c.promptSecret {
		return clientID, "", nil
	}
	if clientID == "" {
		return "", "", trace.BadParameter("--client-secret requires --client-id")
	}

	clientSecret, err := prompt.Password(c.cf.Context, c.cf.Stderr(), prompt.Stdin(), "Enter OAuth client secret")
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	if clientSecret == "" {
		return "", "", trace.BadParameter("OAuth client secret is empty")
	}
	return clientID, clientSecret, nil
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
