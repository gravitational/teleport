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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// defaultMCPOAuthClientName identifies tsh to an MCP server's OAuth provider
// during dynamic client registration. tsh registers under its own name: it is
// tsh that holds the loopback redirect URI, stores the token, and refreshes it,
// and a single stored token is shared by every MCP client that connects
// through it, so no single client's name would describe the credential holder.
const defaultMCPOAuthClientName = "Teleport tsh"

// mcpOAuthClientURI is sent as "client_uri" during registration so providers
// can identify the software behind a dynamically registered client.
const mcpOAuthClientURI = "https://goteleport.com"

type mcpLoginCommand struct {
	*kingpin.CmdClause
	cf           *CLIConf
	clientID     string
	promptSecret bool
	callbackPort uint16
	scopes       []string
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
	cmd.Flag("scope", "OAuth scope to request. This flag can be specified multiple times.").
		StringsVar(&cmd.scopes)
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

	httpClient, err := newMCPOAuthHTTPClient(dialer, app.GetURI())
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

	oauthBaseURL, err := mcpOAuthDiscoveryBaseURL(app.GetURI())
	if err != nil {
		return trace.Wrap(err)
	}

	scopes := c.scopes
	if len(scopes) == 0 {
		advertised, err := fetchAdvertisedMCPOAuthScopes(ctx, httpClient, oauthBaseURL)
		if err != nil {
			logger.DebugContext(ctx, "Failed to fetch advertised OAuth scopes; omitting scope from the authorization request", "error", err)
		} else if len(advertised) > 0 {
			fmt.Fprintf(c.cf.Stdout(), "Requesting scopes advertised by the MCP server: %s\n", strings.Join(advertised, " "))
			scopes = advertised
		}
	}

	tokenStore := mcpclienttransport.NewMemoryTokenStore()
	oauthHandler := mcpclienttransport.NewOAuthHandler(mcpclienttransport.OAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		ClientURI:    mcpOAuthClientURI,
		RedirectURI:  redirectURI,
		Scopes:       scopes,
		PKCEEnabled:  true,
		HTTPClient:   httpClient,
		TokenStore:   tokenStore,
	})
	oauthHandler.SetBaseURL(oauthBaseURL)

	if clientID == "" {
		fmt.Fprintf(c.cf.Stdout(), "Registering OAuth client %q for MCP server %q...\n", defaultMCPOAuthClientName, c.cf.AppSQN.Name)
		if err := oauthHandler.RegisterClient(ctx, defaultMCPOAuthClientName); err != nil {
			return wrapMCPClientRegistrationError(err, c.cf.AppSQN.Name)
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

func fetchAdvertisedMCPOAuthScopes(ctx context.Context, httpClient *http.Client, baseURL string) ([]string, error) {
	metadataURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	metadataURL.Path = "/.well-known/oauth-protected-resource" + strings.TrimSuffix(metadataURL.EscapedPath(), "/")
	metadataURL.RawPath = ""

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL.String(), nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer closeOAuthResponse(resp)
	if resp.StatusCode != http.StatusOK {
		return nil, trace.BadParameter("protected resource metadata request failed with status %v", resp.StatusCode)
	}

	var metadata struct {
		ScopesSupported []string `json:"scopes_supported"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1024*1024)).Decode(&metadata); err != nil {
		return nil, trace.Wrap(err)
	}
	return metadata.ScopesSupported, nil
}

// wrapMCPClientRegistrationError adds instructions for using a pre-registered
// OAuth client when dynamic client registration is not an option: either the
// provider does not offer it at all, or it offers it but refused this client.
func wrapMCPClientRegistrationError(err error, appName string) error {
	switch {
	case err == nil:
		return nil

	// Matched by the error string mcp-go returns when the authorization
	// server metadata has no registration_endpoint.
	case strings.Contains(err.Error(), "does not support dynamic client registration"):
		return trace.Wrap(err, `The MCP server's OAuth provider does not support dynamic client registration.
Register an OAuth client with the provider (or use a public client ID the
provider already publishes), then retry with the pre-registered client:

  tsh mcp login %s --client-id <client-id> --callback-port <port>

Set --callback-port so the redirect URI matches one registered for that client,
and add --client-secret if the client is confidential.`, appName)

	case isMCPClientRegistrationRejected(err):
		return trace.Wrap(err, `The MCP server's OAuth provider rejected the client registration. Some
providers only let an approved set of MCP clients register, so registration
fails even though the provider supports it.

Ask the provider to approve Teleport, or to issue a client ID you can use
directly:

  tsh mcp login %s --client-id <client-id> --callback-port <port>

Set --callback-port so the redirect URI matches one registered for that client,
and add --client-secret if the client is confidential.`, appName)
	}
	return trace.Wrap(err)
}

// isMCPClientRegistrationRejected reports whether the authorization server
// answered the registration request with a refusal, as opposed to failing to
// process it. mcp-go turns an OAuth error body into transport.OAuthError and
// anything else into a message carrying the raw status code.
func isMCPClientRegistrationRejected(err error) bool {
	if _, ok := errors.AsType[mcpclienttransport.OAuthError](err); ok {
		return true
	}
	return strings.Contains(err.Error(), "with status 401") ||
		strings.Contains(err.Error(), "with status 403")
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

// mcpOAuthDiscoveryBaseURL returns the public MCP resource URL used for OAuth
// metadata discovery. hostRoutingTransport sends metadata requests for this
// host through Teleport without exposing its synthetic local routing URL to
// the OAuth handler.
func mcpOAuthDiscoveryBaseURL(appURI string) (string, error) {
	uri, err := url.Parse(appURI)
	if err != nil {
		return "", trace.Wrap(err, "parsing MCP application URI")
	}
	if uri.Scheme != types.SchemeMCPHTTP && uri.Scheme != types.SchemeMCPHTTPS {
		return "", trace.BadParameter("MCP application URI %q does not use HTTP transport", appURI)
	}
	if uri.Host == "" {
		return "", trace.BadParameter("MCP application URI %q is missing a host", appURI)
	}

	uri.Scheme = strings.TrimPrefix(uri.Scheme, "mcp+")
	uri.RawQuery = ""
	uri.Fragment = ""
	return uri.String(), nil
}

// newMCPOAuthHTTPClient returns the HTTP client for talking OAuth. The
// ceremony talks to two different places: the MCP server (for discovery),
// which may only be reachable through the Teleport proxy, and the
// authorization server (for registration, token exchange, and refresh),
// which must be directly reachable since the browser goes there anyway.
// Metadata requests to the MCP server are rewritten to the tunnel's synthetic
// localhost address; authorization and token endpoints go out directly.
func newMCPOAuthHTTPClient(dialer *client.MCPServerDialer, appURI string) (*http.Client, error) {
	oauthBaseURL, err := mcpOAuthDiscoveryBaseURL(appURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	parsedBaseURL, err := url.Parse(oauthBaseURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

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
		Transport: &hostRoutingTransport{
			tunneled:        tunneled,
			direct:          direct,
			mcpServerOrigin: parsedBaseURL,
		},
		Timeout: 30 * time.Second,
	}, nil
}

// hostRoutingTransport sends OAuth metadata requests for the MCP server
// through the Teleport ALPN tunnel and everything else directly. If a
// provider does not implement RFC 9728's path-aware well-known URL, a 404 or
// cross-origin redirect is retried at the root URL for compatibility.
type hostRoutingTransport struct {
	tunneled        http.RoundTripper
	direct          http.RoundTripper
	mcpServerOrigin *url.URL
}

func (t *hostRoutingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := t.roundTrip(r)
	if err != nil || !shouldTryOAuthMetadataRootFallback(r, resp) {
		return resp, err
	}

	retry, ok := t.oauthMetadataRootFallbackRequest(r)
	if !ok {
		return resp, nil
	}

	retryResp, retryErr := t.roundTrip(retry)
	if retryErr != nil || retryResp == nil {
		return resp, nil
	}
	// Preserve a valid cross-origin redirect when the compatibility fallback
	// is unavailable. The HTTP client can still follow the original redirect.
	if isHTTPRedirect(resp.StatusCode) && retryResp.StatusCode >= http.StatusBadRequest {
		closeOAuthResponse(retryResp)
		return resp, nil
	}
	closeOAuthResponse(resp)
	return retryResp, nil
}

func (t *hostRoutingTransport) roundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Hostname() == "localhost" {
		return t.tunneled.RoundTrip(r)
	}
	if sameOAuthOrigin(r.URL, t.mcpServerOrigin) && isOAuthMetadataPath(r.URL.Path) {
		tunneledRequest := r.Clone(r.Context())
		tunneledRequest.URL.Scheme = "http"
		tunneledRequest.URL.Host = "localhost"
		tunneledRequest.Host = ""
		resp, err := t.tunneled.RoundTrip(tunneledRequest)
		if resp != nil {
			resp.Request = r
		}
		return resp, err
	}
	return t.direct.RoundTrip(r)
}

func (t *hostRoutingTransport) oauthMetadataRootFallbackRequest(r *http.Request) (*http.Request, bool) {
	var fallback *http.Request
	for candidate := r; candidate != nil; {
		rootPath, ok := oauthMetadataRootPath(candidate.URL.Path)
		if ok && rootPath != candidate.URL.Path {
			fallback = candidate
			if sameOAuthOrigin(candidate.URL, t.mcpServerOrigin) {
				break
			}
		}
		if candidate.Response == nil {
			break
		}
		candidate = candidate.Response.Request
	}
	if fallback == nil {
		return nil, false
	}

	retry := fallback.Clone(r.Context())
	retry.Response = nil
	retry.URL.Path, _ = oauthMetadataRootPath(fallback.URL.Path)
	retry.URL.RawPath = ""
	return retry, true
}

func shouldTryOAuthMetadataRootFallback(r *http.Request, resp *http.Response) bool {
	if resp == nil || !isOAuthMetadataPath(r.URL.Path) {
		return false
	}
	if resp.StatusCode == http.StatusNotFound {
		return true
	}
	if !isHTTPRedirect(resp.StatusCode) {
		return false
	}
	location, err := resp.Location()
	return err == nil && !sameOAuthOrigin(r.URL, location)
}

func isHTTPRedirect(status int) bool {
	return status >= http.StatusMultipleChoices && status < http.StatusBadRequest
}

func closeOAuthResponse(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 32*1024))
	_ = resp.Body.Close()
}

func sameOAuthOrigin(a, b *url.URL) bool {
	if a == nil || b == nil ||
		!strings.EqualFold(a.Scheme, b.Scheme) ||
		!strings.EqualFold(a.Hostname(), b.Hostname()) {
		return false
	}
	return oauthURLPort(a) == oauthURLPort(b)
}

func oauthURLPort(uri *url.URL) string {
	if port := uri.Port(); port != "" {
		return port
	}
	switch strings.ToLower(uri.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

var oauthMetadataPaths = []string{
	"/.well-known/oauth-protected-resource",
	"/.well-known/oauth-authorization-server",
	"/.well-known/openid-configuration",
}

func isOAuthMetadataPath(path string) bool {
	_, ok := oauthMetadataRootPath(path)
	return ok
}

func oauthMetadataRootPath(path string) (string, bool) {
	for _, root := range oauthMetadataPaths {
		if path == root || strings.HasPrefix(path, root+"/") {
			return root, true
		}
	}
	return "", false
}
