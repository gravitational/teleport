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
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.dny.dev/ssrf"
	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"golang.org/x/net/http/httpproxy"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/sso"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/utils/mcputils"
)

const (
	defaultMCPOAuthClientName    = "Teleport tsh"
	mcpOAuthClientURI            = "https://goteleport.com"
	mcpOAuthAuthorizationTimeout = 3 * time.Minute
	mcpOAuthRefreshLockTimeout   = 30 * time.Second
)

type mcpOAuthReauthorizeFunc func(context.Context, *mcpOAuthCredentials) error

// mcpOAuthLoginConfig configures an interactive login or automatic reauthorization.
type mcpOAuthLoginConfig struct {
	Dialer          *client.MCPServerDialer
	App             types.Application
	CredentialsPath string
	// LockPath serializes credential updates made by login and token refresh.
	LockPath     string
	ClientID     string
	ClientSecret string
	// CallbackPort selects a fixed callback port. When it and RedirectURI are
	// empty, the OS selects an available port.
	CallbackPort uint16
	// RedirectURI is the exact loopback redirect URI to use, from --redirect-uri
	// or an earlier login. It is mutually exclusive with CallbackPort.
	RedirectURI string
	OAuthScopes []string
	// StoredCredentials bind automatic reauthorization to the existing resource and issuer.
	StoredCredentials *mcpOAuthCredentials
	Browser           string
	Stdout            io.Writer
	Stderr            io.Writer
	// Automatic is set when expired stored credentials triggered this login.
	Automatic bool
	// Stdin lets the user paste the redirect URL back when running headless.
	Stdin prompt.Reader
}

type mcpLoginCommand struct {
	*kingpin.CmdClause
	cf           *CLIConf
	clientID     string
	promptSecret bool
	callbackPort uint16
	redirectURI  string
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
	cmd.Flag("redirect-uri", "Loopback OAuth redirect URI registered with the OAuth provider, such as http://localhost:3118/callback. Use it instead of --callback-port when the registered redirect URI does not use 127.0.0.1.").
		StringVar(&cmd.redirectURI)
	cmd.Flag("oauth-scope", "OAuth scopes to request, separated by commas or spaces. This flag can be specified multiple times.").
		StringsVar(&cmd.scopes)
	cmd.Flag("browser", browserHelp).StringVar(&cf.Browser)
	return cmd
}

func (c *mcpLoginCommand) run() error {
	ctx := c.cf.Context
	tc, err := makeClient(c.cf)
	if err != nil {
		return trace.Wrap(err)
	}

	dialer := client.NewMCPServerDialer(tc, c.cf.AppSQN)
	var app types.Application
	err = client.RetryWithRelogin(ctx, tc, func() error {
		app, err = dialer.GetApp(ctx)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if c.applyAppOAuthDefaults(ctx, app) {
		fmt.Fprintf(c.cf.Stdout(), "Using OAuth settings configured for MCP server %q.\n", app.GetName())
	}
	credsPath, err := mcpOAuthTokenPath(c.cf.HomePath, tc.WebProxyHost(), tc.Username, tc.SiteName, c.cf.AppSQN)
	if err != nil {
		return trace.Wrap(err)
	}

	clientID, clientSecret, err := c.getOAuthClientCredentials()
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(runMCPOAuthLogin(ctx, mcpOAuthLoginConfig{
		Dialer:          dialer,
		App:             app,
		CredentialsPath: credsPath,
		LockPath:        mcpOAuthMutationLockPath(c.cf.HomePath),
		ClientID:        clientID,
		ClientSecret:    clientSecret,
		CallbackPort:    c.callbackPort,
		RedirectURI:     c.redirectURI,
		OAuthScopes:     utils.SplitIdentifiers(strings.Join(c.scopes, " ")),
		Browser:         c.cf.Browser,
		Stdout:          c.cf.Stdout(),
		Stderr:          c.cf.Stderr(),
		Stdin:           prompt.Stdin(),
	}))
}

func newMCPOAuthReauthorizeFunc(dialer *client.MCPServerDialer, appName, credentialsPath, mutationLockPath, browser string, output io.Writer) mcpOAuthReauthorizeFunc {
	if browser == teleport.BrowserNone {
		return nil
	}
	if output == nil {
		output = io.Discard
	}
	return func(ctx context.Context, creds *mcpOAuthCredentials) error {
		app, err := dialer.GetApp(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintf(output, "Stored OAuth credentials for MCP server %q have expired; opening a browser to log in again.\n", appName)
		return trace.Wrap(runMCPOAuthLogin(ctx, mcpOAuthLoginConfig{
			Dialer:            dialer,
			App:               app,
			CredentialsPath:   credentialsPath,
			LockPath:          mutationLockPath,
			ClientID:          creds.ClientID,
			ClientSecret:      creds.ClientSecret,
			RedirectURI:       creds.RedirectURI,
			OAuthScopes:       creds.Scopes,
			StoredCredentials: creds,
			Browser:           browser,
			Stdout:            output,
			Stderr:            output,
			Automatic:         true,
		}))
	}
}

// runMCPOAuthLogin implements MCP authorization for HTTP transports.
// https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization
func runMCPOAuthLogin(ctx context.Context, cfg mcpOAuthLoginConfig) error {
	if cfg.Dialer == nil {
		return trace.BadParameter("missing MCP server dialer")
	}
	if cfg.App == nil {
		return trace.BadParameter("missing MCP application")
	}
	if cfg.CredentialsPath == "" {
		return trace.BadParameter("missing MCP OAuth credentials path")
	}
	if cfg.LockPath == "" {
		return trace.BadParameter("missing MCP OAuth mutation lock path")
	}
	if cfg.Stdout == nil {
		cfg.Stdout = io.Discard
	}
	if cfg.Stderr == nil {
		cfg.Stderr = io.Discard
	}
	if types.GetMCPServerTransportType(cfg.App.GetURI()) != types.MCPTransportHTTP {
		return trace.BadParameter("MCP server %q does not use HTTP transport; OAuth login only applies to HTTP MCP servers", cfg.App.GetName())
	}
	appOAuth := appMCPOAuthConfig(cfg.App)
	if appOAuth.Mode == types.MCPOAuthModeNotRequired {
		// because oauth_mode defaults to "not specified", this means that an admin has specifically set
		// to "not required" here, which would mean they are preventing oauth from happening so we can stop trying from here
		fmt.Fprintf(cfg.Stdout, "MCP server %q is configured as not using OAuth, so OAuth login is not required.\n", cfg.App.GetName())
		return nil
	}
	resourceURI := cfg.App.GetURI()
	if cfg.Automatic {
		if cfg.StoredCredentials == nil {
			return trace.BadParameter("missing stored MCP OAuth credentials")
		}
		if err := cfg.StoredCredentials.checkResource(resourceURI, appOAuth.AuthServerMetadataUrl); err != nil {
			return trace.Wrap(err)
		}
	} else {
		// An automatic login runs while refresh already holds this lock.
		// Holding it for the whole flow keeps a manual login from racing an
		// automatic one in another process and stalling after its browser step.
		if err := os.MkdirAll(filepath.Dir(cfg.CredentialsPath), 0o700); err != nil {
			return trace.ConvertSystemError(err)
		}
		unlock, err := utils.FSTryWriteLockTimeout(ctx, cfg.CredentialsPath+".lock", mcpOAuthAuthorizationTimeout+mcpOAuthRefreshLockTimeout)
		if err != nil {
			return trace.Wrap(err, "waiting for another tsh process to finish refreshing or logging in")
		}
		defer unlock()
	}

	httpClient, err := newMCPOAuthHTTPClient(cfg.Dialer, resourceURI, appOAuth.AuthServerMetadataUrl)
	if err != nil {
		return trace.Wrap(err)
	}

	// Registration must use the exact callback address that is already bound.
	listener, redirectURI, err := listenMCPOAuthCallback(cfg.CallbackPort, cfg.RedirectURI)
	if err != nil {
		return trace.Wrap(err)
	}
	defer listener.Close()

	oauthBaseURL, err := mcpOAuthDiscoveryBaseURL(resourceURI)
	if err != nil {
		return trace.Wrap(err)
	}

	status, required, err := fetchMCPOAuthChallengeScopes(ctx, httpClient, oauthBaseURL)
	switch {
	case err != nil:
		logger.DebugContext(ctx, "Failed to probe the MCP server for an OAuth challenge", "error", err)
	case status >= http.StatusOK && status < http.StatusMultipleChoices && cfg.ClientID == "" && appOAuth.Mode != types.MCPOAuthModeRequired:
		// Some servers, such as Gmail's, answer an unauthenticated initialize
		// request and require auth only for tool calls. Their protected resource
		// metadata shows they still use OAuth. An app configured as requiring
		// OAuth always logs in.
		if _, err := fetchAdvertisedMCPOAuthScopes(ctx, httpClient, oauthBaseURL); err != nil {
			logger.DebugContext(ctx, "Failed to fetch OAuth protected resource metadata", "error", err)
			fmt.Fprintf(cfg.Stdout, "MCP server %q accepted an unauthenticated request and publishes no OAuth protected resource metadata, so OAuth login is not required.\n", cfg.App.GetName())
			return nil
		}
	}
	scopes := cfg.OAuthScopes
	if len(scopes) == 0 && len(required) > 0 {
		fmt.Fprintf(cfg.Stdout, "Requesting scopes required by the MCP server: %s\n", strings.Join(required, " "))
		scopes = required
	}
	if len(scopes) == 0 {
		advertised, err := fetchAdvertisedMCPOAuthScopes(ctx, httpClient, oauthBaseURL)
		if err != nil {
			logger.DebugContext(ctx, "Failed to fetch advertised OAuth scopes; omitting scope from the authorization request", "error", err)
		} else if len(advertised) > 0 {
			fmt.Fprintf(cfg.Stdout, "Requesting scopes advertised by the MCP server: %s\n", strings.Join(advertised, " "))
			scopes = advertised
		}
	}

	clientID, clientSecret := cfg.ClientID, cfg.ClientSecret
	var registrationMetadata *mcpclienttransport.AuthServerMetadata
	if clientID == "" {
		fmt.Fprintf(cfg.Stdout, "Registering OAuth client %q for MCP server %q...\n", defaultMCPOAuthClientName, cfg.App.GetName())
		discoveryHandler := mcpclienttransport.NewOAuthHandler(mcpclienttransport.OAuthConfig{HTTPClient: httpClient})
		discoveryHandler.SetBaseURL(oauthBaseURL)
		registrationMetadata, err = getMCPOAuthServerMetadata(ctx, discoveryHandler)
		if err != nil {
			return trace.Wrap(err, "failed to get server metadata")
		}
		clientID, clientSecret, err = registerMCPOAuthClient(ctx, registrationMetadata, httpClient, redirectURI, scopes)
		if err != nil {
			return wrapMCPClientRegistrationError(err, cfg.App.GetName())
		}
	} else if cfg.Automatic {
		fmt.Fprintf(cfg.Stdout, "Reusing the stored OAuth client and callback address for MCP server %q...\n", cfg.App.GetName())
	} else {
		fmt.Fprintf(cfg.Stdout, "Using pre-registered OAuth client for MCP server %q...\n", cfg.App.GetName())
	}

	tokenStore := mcpclienttransport.NewMemoryTokenStore()
	oauthHandler := mcpclienttransport.NewOAuthHandler(mcpclienttransport.OAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		Scopes:       scopes,
		PKCEEnabled:  true,
		HTTPClient:   httpClient,
		TokenStore:   tokenStore,
	})
	oauthHandler.SetBaseURL(oauthBaseURL)
	metadata, err := getMCPOAuthServerMetadata(ctx, oauthHandler)
	if err != nil {
		return trace.Wrap(err)
	}
	if registrationMetadata != nil {
		if err := checkMCPOAuthServerMetadataUnchanged(registrationMetadata, metadata); err != nil {
			return trace.Wrap(err)
		}
	}
	if cfg.Automatic {
		// Automatic login reuses stored client credentials, which must stay
		// bound to the same resource, metadata URL and issuer. An explicit
		// login establishes a new binding.
		if err := cfg.StoredCredentials.checkBinding(resourceURI, appOAuth.AuthServerMetadataUrl, metadata.Issuer); err != nil {
			return trace.Wrap(err)
		}
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
	if err := validateMCPOAuthDirectURL(ctx, authURL, "authorization endpoint"); err != nil {
		return trace.Wrap(err)
	}

	callbackCh := make(chan url.Values, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", mcpOAuthCallbackHandler(state, callbackCh))
	callbackServer := &http.Server{
		Handler:           mux,
		ReadTimeout:       apidefaults.DefaultIOTimeout,
		ReadHeaderTimeout: defaults.ReadHeadersTimeout,
		WriteTimeout:      apidefaults.DefaultIOTimeout,
		IdleTimeout:       apidefaults.DefaultIdleTimeout,
	}
	go callbackServer.Serve(listener)
	defer callbackServer.Close()

	fmt.Fprintf(cfg.Stdout, "Opening browser for authorization. If it does not open, visit:\n\n  %v\n\n", authURL)
	if err := sso.OpenURLInBrowser(cfg.Browser, authURL); err != nil {
		fmt.Fprintf(cfg.Stderr, "Failed to open a browser: %v\n", err)
	}

	if cfg.Stdin != nil {
		pasteCtx, cancelPaste := context.WithCancel(ctx)
		defer cancelPaste()
		fmt.Fprintln(cfg.Stdout, "If the browser cannot reach this machine, paste the URL it was redirected to:")
		go readMCPOAuthPastedRedirect(pasteCtx, cfg.Stdin, cfg.Stdout, state, callbackCh)
	}

	var query url.Values
	select {
	case query = <-callbackCh:
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	case <-time.After(mcpOAuthAuthorizationTimeout):
		return trace.LimitExceeded("timed out waiting for the browser authorization to complete")
	}
	if errCode := query.Get("error"); errCode != "" {
		return trace.AccessDenied("authorization failed: %v: %v", errCode, query.Get("error_description"))
	}
	if err := checkMCPOAuthCallbackIssuer(query, metadata.Issuer); err != nil {
		return trace.Wrap(err)
	}

	if err := oauthHandler.ProcessAuthorizationResponse(ctx, query.Get("code"), query.Get("state"), codeVerifier); err != nil {
		return trace.Wrap(err)
	}

	token, err := tokenStore.GetToken(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := saveMCPOAuthLoginCredentials(ctx, cfg.CredentialsPath, cfg.LockPath, cfg.Automatic, &mcpOAuthCredentials{
		ResourceURI:           resourceURI,
		AuthServerMetadataURL: appOAuth.AuthServerMetadataUrl,
		Issuer:                metadata.Issuer,
		ClientID:              oauthHandler.GetClientID(),
		ClientSecret:          oauthHandler.GetClientSecret(),
		RedirectURI:           redirectURI,
		Scopes:                scopes,
		Token:                 *token,
	}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintf(cfg.Stdout, "Authorization complete. Tokens stored in %v.\n", cfg.CredentialsPath)
	if cfg.Automatic {
		fmt.Fprintf(cfg.Stdout, "MCP server %q is ready; continuing the connection.\n", cfg.App.GetName())
	} else {
		fmt.Fprintf(cfg.Stdout, "MCP server %q is ready — restart your MCP clients if already running.\n", cfg.App.GetName())
	}
	return nil
}

// mcpOAuthCallbackHandler keeps the first authorization response that carries
// the expected state and drops everything else. Rejecting other requests up
// front means a stray hit on the callback port can neither take the buffered
// slot from the real response nor block the handler once nobody is reading.
func mcpOAuthCallbackHandler(state string, callbackCh chan<- url.Values) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("state") != state {
			http.Error(w, "invalid state", http.StatusBadRequest)
			return
		}
		select {
		case callbackCh <- query:
			fmt.Fprintln(w, "Login complete. You can close this tab and return to the terminal.")
		default:
			fmt.Fprintln(w, "An authorization response has already been received. Return to the terminal to check the login result.")
		}
	}
}

// readMCPOAuthPastedRedirect lets a headless user finish the login by pasting
// the URL the browser was redirected to, such as
// http://127.0.0.1:3118/callback?code=...&state=...
// Like mcpOAuthCallbackHandler, it only accepts a URL with the expected state.
func readMCPOAuthPastedRedirect(ctx context.Context, in prompt.Reader, out io.Writer, state string, callbackCh chan<- url.Values) {
	for {
		line, err := in.ReadContext(ctx)
		if err != nil {
			return
		}
		pasted := strings.TrimSpace(string(line))
		if pasted == "" {
			continue
		}
		pastedURL, err := url.Parse(pasted)
		if err != nil || pastedURL.Query().Get("state") != state {
			fmt.Fprintln(out, "That URL does not match this login. Paste the URL the browser was redirected to:")
			continue
		}
		select {
		case callbackCh <- pastedURL.Query():
		default:
		}
		return
	}
}

// checkMCPOAuthCallbackIssuer enforces RFC 9207: when the authorization
// response names the issuer that authorized the code, it must be the issuer
// whose token endpoint is about to receive that code.
// https://www.rfc-editor.org/rfc/rfc9207.html
func checkMCPOAuthCallbackIssuer(query url.Values, issuer string) error {
	iss := query.Get("iss")
	if iss == "" || iss == issuer {
		return nil
	}
	return trace.AccessDenied("authorization response came from issuer %q, expected %q", iss, issuer)
}

func saveMCPOAuthLoginCredentials(ctx context.Context, path, mutationLockPath string, automatic bool, creds *mcpOAuthCredentials) error {
	return withMCPOAuthMutationLock(ctx, mutationLockPath, func() error {
		// An automatic login must not recreate credentials that logout removed meanwhile.
		if automatic {
			if _, err := loadMCPOAuthCredentials(path); err != nil {
				return trace.Wrap(err)
			}
		}
		return trace.Wrap(saveMCPOAuthCredentials(path, creds))
	})
}

func listenMCPOAuthCallback(callbackPort uint16, redirectURI string) (net.Listener, string, error) {
	if redirectURI != "" {
		if callbackPort != 0 {
			return nil, "", trace.BadParameter("callback port and redirect URI are mutually exclusive")
		}
		parsed, err := url.Parse(redirectURI)
		if err != nil {
			return nil, "", trace.Wrap(err, "parsing MCP OAuth redirect URI")
		}
		hostname := parsed.Hostname()
		isLoopback := hostname == "localhost" || net.ParseIP(hostname).IsLoopback()
		if parsed.String() != fmt.Sprintf("http://%s/callback", parsed.Host) || parsed.Port() == "" || !isLoopback {
			return nil, "", trace.BadParameter("MCP OAuth redirect URI %q is not a loopback callback URL, such as http://localhost:3118/callback", redirectURI)
		}
		listener, err := net.Listen("tcp", parsed.Host)
		if err != nil {
			return nil, "", trace.Wrap(err, "binding MCP OAuth callback address %q", parsed.Host)
		}
		return listener, parsed.String(), nil
	}

	listenAddr := fmt.Sprintf("127.0.0.1:%d", callbackPort)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	redirectURI = fmt.Sprintf("http://%s/callback", listener.Addr())
	return listener, redirectURI, nil
}

// fetchMCPOAuthChallengeScopes sends an unauthenticated initialize request to
// discover required OAuth scopes from a 401 Bearer challenge.
func fetchMCPOAuthChallengeScopes(ctx context.Context, httpClient *http.Client, resourceURL string) (statusCode int, scopes []string, err error) {
	// TODO (avatus) Let the official MCP SDK probe with server/discover and fall back
	// to initialize once this client is migrated to it.
	body, err := json.Marshal(mcpclienttransport.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(1),
		Method:  string(mcp.MethodInitialize),
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			Capabilities:    mcp.ClientCapabilities{},
			ClientInfo: mcp.Implementation{
				Name:    defaultMCPOAuthClientName,
				Version: teleport.Version,
			},
		},
	})
	if err != nil {
		return 0, nil, trace.Wrap(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resourceURL, bytes.NewReader(body))
	if err != nil {
		return 0, nil, trace.Wrap(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, nil, trace.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		return resp.StatusCode, nil, nil
	}
	// Parse `Bearer realm="mcp", scope="mcp:tools mcp:resources"` and
	// return the space-delimited values in its quoted scope parameter.
	for _, challenge := range resp.Header.Values("WWW-Authenticate") {
		scheme, params, ok := strings.Cut(strings.TrimSpace(challenge), " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") {
			continue
		}
		for params != "" {
			name, value, ok := strings.Cut(strings.TrimLeft(params, " \t,"), "=")
			if !ok {
				break
			}
			value = strings.TrimLeft(value, " \t")
			if !strings.HasPrefix(value, `"`) {
				_, params, _ = strings.Cut(value, ",")
				continue
			}
			quoted, err := strconv.QuotedPrefix(value)
			if err != nil {
				break
			}
			if strings.EqualFold(strings.TrimSpace(name), "scope") {
				scope, err := strconv.Unquote(quoted)
				if err == nil {
					return resp.StatusCode, strings.Fields(scope), nil
				}
				break
			}
			params = value[len(quoted):]
		}
	}
	return resp.StatusCode, nil, nil
}

func fetchAdvertisedMCPOAuthScopes(ctx context.Context, httpClient *http.Client, baseURL string) ([]string, error) {
	metadataURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	metadataURL.Path = mcputils.OAuthProtectedResourceMetadataPath + strings.TrimSuffix(metadataURL.EscapedPath(), "/")
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
	if err := json.NewDecoder(io.LimitReader(resp.Body, teleport.MaxHTTPResponseSize)).Decode(&metadata); err != nil {
		return nil, trace.Wrap(err)
	}
	return metadata.ScopesSupported, nil
}

func registerMCPOAuthClient(ctx context.Context, metadata *mcpclienttransport.AuthServerMetadata, httpClient *http.Client, redirectURI string, scopes []string) (string, string, error) {
	if metadata.RegistrationEndpoint == "" {
		return "", "", errors.New("server does not support dynamic client registration")
	}

	registration := map[string]any{
		"client_name":                defaultMCPOAuthClientName,
		"client_uri":                 mcpOAuthClientURI,
		"redirect_uris":              []string{redirectURI},
		"token_endpoint_auth_method": "none",
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
	}
	if len(scopes) > 0 {
		registration["scope"] = strings.Join(scopes, " ")
	}
	body, err := json.Marshal(registration)
	if err != nil {
		return "", "", trace.Wrap(err, "failed to marshal registration request")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, metadata.RegistrationEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", "", trace.Wrap(err, "failed to create registration request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", trace.Wrap(err, "failed to send registration request")
	}
	defer closeOAuthResponse(resp)
	body, err = io.ReadAll(io.LimitReader(resp.Body, teleport.MaxHTTPResponseSize))
	if err != nil {
		return "", "", trace.Wrap(err, "failed to read registration response")
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		var oauthErr mcpclienttransport.OAuthError
		if err := json.Unmarshal(body, &oauthErr); err == nil && oauthErr.ErrorCode != "" {
			return "", "", trace.Wrap(oauthErr, "registration request failed")
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return "", "", trace.AccessDenied("registration request failed with status %d: %s", resp.StatusCode, body)
		}
		return "", "", trace.Errorf("registration request failed with status %d: %s", resp.StatusCode, body)
	}

	var registrationResponse struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret,omitempty"`
	}
	if err := json.Unmarshal(body, &registrationResponse); err != nil {
		return "", "", trace.Wrap(err, "failed to decode registration response")
	}
	return registrationResponse.ClientID, registrationResponse.ClientSecret, nil
}

func checkMCPOAuthServerMetadataUnchanged(registered, current *mcpclienttransport.AuthServerMetadata) error {
	if registered.Issuer != current.Issuer ||
		registered.AuthorizationEndpoint != current.AuthorizationEndpoint ||
		registered.TokenEndpoint != current.TokenEndpoint {
		return trace.BadParameter("OAuth authorization server metadata changed during client registration")
	}
	return nil
}

func wrapMCPClientRegistrationError(err error, appName string) error {
	switch {
	case strings.Contains(err.Error(), "does not support dynamic client registration"):
		return trace.Wrap(err, `The MCP server's OAuth provider does not support dynamic client registration.
Register an OAuth client with the provider (or use a public client ID the
provider already publishes), then retry with the pre-registered client:

  tsh mcp login %s --client-id <client-id> --callback-port <port>

Set --callback-port so the redirect URI matches one registered for that client,
and add --client-secret if the client is confidential.`, appName)

	case isMCPInvalidClientMetadata(err):
		return trace.Wrap(err, `The client metadata sent by tsh is invalid for the MCP server's OAuth
provider. This indicates a dynamic client registration compatibility problem,
not a provider client allowlist rejection.`)

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

func isMCPInvalidClientMetadata(err error) bool {
	var oauthErr mcpclienttransport.OAuthError
	return errors.As(err, &oauthErr) && oauthErr.ErrorCode == "invalid_client_metadata"
}

func isMCPClientRegistrationRejected(err error) bool {
	var oauthErr mcpclienttransport.OAuthError
	if errors.As(err, &oauthErr) {
		return true
	}
	return trace.IsAccessDenied(err)
}

// applyAppOAuthDefaults fills the OAuth settings that were not set with flags
// from the app's mcp.oauth configuration, and reports whether it used any.
func (c *mcpLoginCommand) applyAppOAuthDefaults(ctx context.Context, app types.Application) bool {
	appOAuth := appMCPOAuthConfig(app)
	var applied []slog.Attr
	if c.clientID == "" && appOAuth.ClientId != "" {
		c.clientID = appOAuth.ClientId
		applied = append(applied, slog.String("client_id", c.clientID))
	}
	// Both flags set the callback address, so either one replaces the app's.
	// this gives the user the option to override these with one or the other
	// instead of hitting the mututally exclusive error, if they choose.
	if c.callbackPort == 0 && c.redirectURI == "" {
		if appOAuth.CallbackPort != 0 {
			c.callbackPort = uint16(appOAuth.CallbackPort)
			applied = append(applied, slog.Int("callback_port", int(c.callbackPort)))
		}
		if appOAuth.RedirectUri != "" {
			c.redirectURI = appOAuth.RedirectUri
			applied = append(applied, slog.String("redirect_uri", c.redirectURI))
		}
	}
	if len(c.scopes) == 0 && len(appOAuth.Scopes) > 0 {
		c.scopes = appOAuth.Scopes
		applied = append(applied, slog.String("scopes", strings.Join(c.scopes, " ")))
	}
	if len(applied) > 0 {
		logger.LogAttrs(ctx, slog.LevelDebug, "Using OAuth settings from the MCP server's app configuration", applied...)
	}
	return len(applied) > 0
}

// appMCPOAuthConfig returns the app's mcp.oauth configuration, or an empty one
// when the app has none.
func appMCPOAuthConfig(app types.Application) *types.MCPOAuth {
	if mcp := app.GetMCP(); mcp != nil && mcp.Oauth != nil {
		return mcp.Oauth
	}
	return &types.MCPOAuth{}
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

var mcpOAuthNetworkGuard = ssrf.New(ssrf.WithAnyPort())

type mcpOAuthDiscoveryState struct {
	advertisedAuthorizationServer string
	// discoveredIssuer is the issuer of the authorization server metadata
	// document that passed validation, if any.
	discoveredIssuer string
	// protectedResourceErrorStatus is the status of a protected resource
	// metadata response that failed with anything other than 404 Not Found.
	protectedResourceErrorStatus int
}

type mcpOAuthDiscoveryStateKey struct{}

func getMCPOAuthServerMetadata(ctx context.Context, handler *mcpclienttransport.OAuthHandler) (*mcpclienttransport.AuthServerMetadata, error) {
	discovery := new(mcpOAuthDiscoveryState)
	metadata, err := handler.GetServerMetadata(context.WithValue(ctx, mcpOAuthDiscoveryStateKey{}, discovery))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if metadata == nil {
		return nil, trace.BadParameter("OAuth authorization server metadata is empty")
	}
	// mcp-go guesses root endpoints when discovery fails. we keep those endpoints
	// bound to the authorization server selected by the protected resource.
	if advertised := discovery.advertisedAuthorizationServer; advertised != "" && strings.TrimSuffix(metadata.Issuer, "/") != strings.TrimSuffix(advertised, "/") {
		return nil, trace.BadParameter("OAuth authorization server metadata identifies issuer %q, but protected resource metadata advertised %q", metadata.Issuer, advertised)
	}
	// mcp-go guessed this metadata after the protected resource metadata
	// request failed. Its guessed endpoints would fail later with an error
	// that hides the real one.
	if status := discovery.protectedResourceErrorStatus; status != 0 && metadata.Issuer != discovery.discoveredIssuer {
		return nil, trace.Errorf("MCP server answered its OAuth protected resource metadata request with status %d %s", status, http.StatusText(status))
	}
	for _, endpoint := range []struct {
		name     string
		url      string
		required bool
	}{
		{name: "issuer", url: metadata.Issuer, required: true},
		{name: "authorization endpoint", url: metadata.AuthorizationEndpoint, required: true},
		{name: "token endpoint", url: metadata.TokenEndpoint, required: true},
		{name: "registration endpoint", url: metadata.RegistrationEndpoint},
	} {
		if endpoint.url == "" && !endpoint.required {
			continue
		}
		if _, err := parseMCPOAuthDirectURL(endpoint.url, endpoint.name); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	logger.Log(ctx, logutils.TraceLevel, "Discovered MCP OAuth server metadata", "metadata", metadata)
	return metadata, nil
}

func parseMCPOAuthDirectURL(rawURL, name string) (*url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, trace.Wrap(err, "parsing OAuth %s URL", name)
	}
	if !strings.EqualFold(parsed.Scheme, "https") || parsed.Host == "" || parsed.User != nil {
		return nil, trace.BadParameter("OAuth %s must be an absolute HTTPS URL", name)
	}
	if ip := net.ParseIP(parsed.Hostname()); ip != nil {
		network := "tcp6"
		if ip.To4() != nil {
			network = "tcp4"
		}
		if err := mcpOAuthNetworkGuard.Safe(network, net.JoinHostPort(ip.String(), "443"), nil); err != nil {
			return nil, trace.BadParameter("OAuth %s uses a prohibited network address", name)
		}
	}
	return parsed, nil
}

// validateMCPOAuthDirectURL checks a URL that will be fetched by something
// other than the guarded direct transport like the browser opening the
// authorization URL, or an HTTPS proxy dialing on tsh's behalf. The dial-time
// SSRF guard never sees those connections, so the host is resolved here and
// every address it resolves to is run through the same guard instead.
func validateMCPOAuthDirectURL(ctx context.Context, rawURL, name string) error {
	parsed, err := parseMCPOAuthDirectURL(rawURL, name)
	if err != nil {
		return trace.Wrap(err)
	}
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", parsed.Hostname())
	if err != nil {
		return trace.Wrap(err, "resolving OAuth %s", name)
	}
	port := cmp.Or(parsed.Port(), "443")
	// A hostname may resolve to multiple addresses; reject it if any
	// destination is prohibited.
	for _, ip := range addresses {
		ip = ip.Unmap()
		network := "tcp6"
		if ip.Is4() {
			network = "tcp4"
		}
		if err := mcpOAuthNetworkGuard.Safe(network, net.JoinHostPort(ip.String(), port), nil); err != nil {
			return trace.BadParameter("OAuth %s resolves to a prohibited network address", name)
		}
	}
	return nil
}
func newMCPOAuthHTTPClient(dialer *client.MCPServerDialer, appURI, authServerMetadataURL string) (*http.Client, error) {
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
	// Every request goes through the Teleport tunnel, never an HTTP proxy.
	tunneled.Proxy = nil
	tunneled.DialContext = dialer.DialContext
	direct, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	direct.Proxy = nil
	direct.DialContext = (&net.Dialer{Control: mcpOAuthNetworkGuard.Safe}).DialContext
	// An HTTPS_PROXY dials the target itself, so the dial guard cannot inspect
	// it. Proxied requests are checked by resolved address before sending.
	proxied, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	proxyFunc := httpproxy.FromEnvironment().ProxyFunc()
	proxyFromEnvironment := func(req *http.Request) (*url.URL, error) {
		return proxyFunc(req.URL)
	}
	proxied.Proxy = proxyFromEnvironment
	return &http.Client{
		Transport: &hostRoutingTransport{
			tunneled:              tunneled,
			direct:                direct,
			proxied:               proxied,
			proxy:                 proxyFromEnvironment,
			mcpServerOrigin:       parsedBaseURL,
			authServerMetadataURL: authServerMetadataURL,
		},
		// Token and registration requests carry credentials in their bodies,
		// which a 307 or 308 would replay to the redirect target.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if via[0].Method != http.MethodGet {
				return trace.BadParameter("refusing to follow a redirect for an OAuth %s request", via[0].Method)
			}
			if len(via) >= 10 {
				return trace.LimitExceeded("stopped after 10 redirects")
			}
			return nil
		},
		Timeout: 30 * time.Second,
	}, nil
}

// hostRoutingTransport lets the MCP OAuth flow use one HTTP client for two
// network paths. MCP server requests use MCPServerDialer; OAuth provider requests
// use direct HTTPS or HTTPS_PROXY. It also handles metadata fallbacks, validates
// discovery metadata, and bounds response bodies.
type hostRoutingTransport struct {
	tunneled        http.RoundTripper
	direct          http.RoundTripper
	proxied         http.RoundTripper
	proxy           func(*http.Request) (*url.URL, error)
	mcpServerOrigin *url.URL
	// authServerMetadataURL is the authorization server metadata URL configured
	// for the MCP server. When set, it answers every authorization server
	// metadata request, and its issuer is not checked against the URL.
	authServerMetadataURL string
}

func (t *hostRoutingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if t.authServerMetadataURL != "" && isAuthServerMetadataRequest(r) {
		return t.configuredAuthServerMetadata(r)
	}
	resp, err := t.roundTrip(r)
	if err != nil {
		return resp, err
	}

	if shouldTryOAuthMetadataRootFallback(r, resp) {
		if retry := t.oauthMetadataRootFallbackRequest(r); retry != nil {
			retryResp, retryErr := t.roundTrip(retry)
			if retryErr == nil && retryResp != nil {
				// Keep a usable discovery redirect when its compatibility fallback fails.
				if isHTTPRedirect(resp.StatusCode) && retryResp.StatusCode >= http.StatusBadRequest {
					closeOAuthResponse(retryResp)
				} else {
					closeOAuthResponse(resp)
					resp = retryResp
				}
			}
		}
	}

	// mcp-go only tries /.well-known/openid-configuration/<issuer path>, but some
	// providers only serve <issuer path>/.well-known/openid-configuration.
	if retry := openIDConfigurationAppendedRequest(r, resp); retry != nil {
		retryResp, retryErr := t.roundTrip(retry)
		if retryErr == nil && retryResp != nil {
			closeOAuthResponse(resp)
			resp = retryResp
		}
	}

	if err := validateMCPOAuthMetadataResponse(r, resp, t.mcpServerOrigin.String(), t.authServerMetadataURL); err != nil {
		closeOAuthResponse(resp)
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// isAuthServerMetadataRequest reports whether r starts a request for
// authorization server or OpenID Connect metadata.
func isAuthServerMetadataRequest(r *http.Request) bool {
	root, ok := oauthMetadataRootPath(r.URL.Path)
	return ok && r.Method == http.MethodGet && r.Response == nil && root != mcputils.OAuthProtectedResourceMetadataPath
}

// configuredAuthServerMetadata answers an authorization server metadata
// request with the configured document. mcp-go still discovers protected
// resource metadata, which sets the RFC 8707 resource indicator.
func (t *hostRoutingTransport) configuredAuthServerMetadata(r *http.Request) (*http.Response, error) {
	logger.DebugContext(r.Context(), "Using the configured OAuth authorization server metadata URL", "requested", r.URL.String(), "configured", t.authServerMetadataURL)
	metadataURL, err := url.Parse(t.authServerMetadataURL)
	if err != nil {
		return nil, trace.Wrap(err, "parsing OAuth authorization server metadata URL")
	}
	configured := r.Clone(r.Context())
	configured.URL = metadataURL
	configured.Host = ""
	resp, err := t.roundTrip(configured)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if isHTTPRedirect(resp.StatusCode) {
		closeOAuthResponse(resp)
		return nil, trace.BadParameter("OAuth authorization server metadata URL %q redirected; configure the final URL", t.authServerMetadataURL)
	}
	if resp.StatusCode != http.StatusOK {
		closeOAuthResponse(resp)
		return nil, trace.BadParameter("OAuth authorization server metadata URL %q returned status %d %s", t.authServerMetadataURL, resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	resp.Request = r
	// Records the document's issuer for the protected resource error check.
	if err := validateMCPOAuthMetadataResponse(r, resp, t.mcpServerOrigin.String(), t.authServerMetadataURL); err != nil {
		closeOAuthResponse(resp)
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

func validateMCPOAuthMetadataResponse(req *http.Request, resp *http.Response, expectedResource, authServerMetadataURL string) error {
	metadataURL, rootPath, ok := oauthMetadataChainOrigin(req)
	if !ok || resp == nil {
		return nil
	}
	discovery, _ := req.Context().Value(mcpOAuthDiscoveryStateKey{}).(*mcpOAuthDiscoveryState)
	// mcp-go guesses default endpoints whenever this request fails. That's fine
	// for a 404, but for errors like a 403 we record the status so login can
	// fail with a clear error instead.
	if resp.StatusCode != http.StatusOK {
		if discovery != nil && rootPath == mcputils.OAuthProtectedResourceMetadataPath &&
			resp.StatusCode >= http.StatusBadRequest && resp.StatusCode != http.StatusNotFound {
			discovery.protectedResourceErrorStatus = resp.StatusCode
		}
		return nil
	}

	originalBody := resp.Body
	defer originalBody.Close()

	body, err := utils.ReadAtMost(originalBody, teleport.MaxHTTPResponseSize)
	if err != nil {
		return trace.Wrap(err, "reading OAuth metadata")
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))

	var metadata struct {
		Resource             string   `json:"resource"`
		Issuer               string   `json:"issuer"`
		AuthorizationServers []string `json:"authorization_servers"`
	}
	if err := json.Unmarshal(body, &metadata); err != nil {
		return trace.Wrap(err, "decoding OAuth metadata")
	}
	if rootPath == mcputils.OAuthProtectedResourceMetadataPath {
		if !mcpOAuthResourceCoversServer(metadata.Resource, expectedResource) {
			return trace.BadParameter("OAuth protected resource metadata identifies resource %q, expected %q or a parent of it", metadata.Resource, expectedResource)
		}
		// A configured metadata document replaces the advertised server.
		if discovery != nil && authServerMetadataURL == "" && len(metadata.AuthorizationServers) > 0 {
			discovery.advertisedAuthorizationServer = metadata.AuthorizationServers[0]
		}
		return nil
	}
	// The admin chose this document, so its issuer may differ from its URL.
	if authServerMetadataURL != "" {
		if discovery != nil {
			discovery.discoveredIssuer = metadata.Issuer
		}
		return nil
	}
	// RFC 8414 section 3.3: the issuer must match the URL the metadata was
	// discovered from, so a document cannot impersonate another provider.
	// https://www.rfc-editor.org/rfc/rfc8414.html#section-3.3
	expectedIssuer := metadataURL.Scheme + "://" + metadataURL.Host + oauthMetadataIssuerPath(metadataURL.EscapedPath(), rootPath)
	if strings.TrimSuffix(metadata.Issuer, "/") != strings.TrimSuffix(expectedIssuer, "/") {
		return trace.BadParameter("OAuth authorization server metadata identifies issuer %q, expected %q", metadata.Issuer, expectedIssuer)
	}
	if discovery != nil {
		discovery.discoveredIssuer = metadata.Issuer
	}
	return nil
}

// mcpOAuthResourceCoversServer reports whether a protected resource metadata
// resource identifier is the MCP server URL or a parent of it on the same
// origin. RFC 9728 section 3.3 asks for an exact match, but servers commonly
// publish one document for the whole origin, and the official MCP SDKs accept
// that. Pinning the origin still keeps metadata from naming another service as
// the token audience.
// https://www.rfc-editor.org/rfc/rfc9728.html#section-3.3
func mcpOAuthResourceCoversServer(resource, serverURL string) bool {
	parsedResource, err := url.Parse(resource)
	if err != nil {
		return false
	}
	parsedServerURL, err := url.Parse(serverURL)
	if err != nil {
		return false
	}
	if !utils.SameHTTPOrigin(parsedResource, parsedServerURL) {
		return false
	}
	// Compare whole path segments so "/mcp" does not cover "/mcp2".
	resourcePath := strings.TrimSuffix(parsedResource.EscapedPath(), "/") + "/"
	serverPath := strings.TrimSuffix(parsedServerURL.EscapedPath(), "/") + "/"
	return strings.HasPrefix(serverPath, resourcePath)
}

func (t *hostRoutingTransport) roundTrip(r *http.Request) (*http.Response, error) {
	metadataRoot, _ := oauthMetadataRootPath(r.URL.Path)
	// The MCP endpoint and its protected-resource metadata belong to the
	// registered application and therefore use its Teleport tunnel.
	// App Service follows same-origin protected-resource metadata redirects.
	if utils.SameHTTPOrigin(r.URL, t.mcpServerOrigin) &&
		(r.URL.EscapedPath() == t.mcpServerOrigin.EscapedPath() || metadataRoot == mcputils.OAuthProtectedResourceMetadataPath) {
		logger.Log(r.Context(), logutils.TraceLevel, "Routing MCP OAuth request",
			"transport", "tunnel", "method", r.Method,
			"host", r.URL.Host, "path", r.URL.EscapedPath())
		tunneledRequest := r.Clone(r.Context())
		tunneledRequest.URL.Scheme = "http"
		resp, err := t.tunneled.RoundTrip(tunneledRequest)
		if resp != nil {
			resp.Request = r
		}
		if err != nil {
			return resp, err
		}
		return boundOAuthResponse(resp), nil
	}
	// Authorization-server endpoints use the workstation's normal HTTPS path,
	// even when they share the MCP resource's origin.
	if !strings.EqualFold(r.URL.Scheme, "https") {
		return nil, trace.BadParameter("OAuth requests require HTTPS")
	}
	transport := t.direct
	transportName := "direct"
	if t.proxy != nil {
		proxyURL, err := t.proxy(r)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if proxyURL != nil {
			if err := validateMCPOAuthDirectURL(r.Context(), r.URL.String(), "endpoint"); err != nil {
				return nil, trace.Wrap(err)
			}
			transport = t.proxied
			transportName = "proxy"
		}
	}
	logger.Log(r.Context(), logutils.TraceLevel, "Routing MCP OAuth request",
		"transport", transportName, "method", r.Method,
		"host", r.URL.Host, "path", r.URL.EscapedPath())
	resp, err := transport.RoundTrip(r)
	if err != nil {
		return resp, err
	}
	return boundOAuthResponse(resp), nil
}

// boundOAuthResponse caps a response body because mcp-go decodes token and
// metadata responses without a size bound.
func boundOAuthResponse(resp *http.Response) *http.Response {
	resp.Body = struct {
		io.Reader
		io.Closer
	}{utils.LimitReader(resp.Body, teleport.MaxHTTPResponseSize), resp.Body}
	return resp
}

func (t *hostRoutingTransport) oauthMetadataRootFallbackRequest(r *http.Request) *http.Request {
	var fallback *http.Request
	for candidate, rootPath := range oauthMetadataRequestChain(r) {
		if rootPath != candidate.URL.Path {
			fallback = candidate
			if utils.SameHTTPOrigin(candidate.URL, t.mcpServerOrigin) {
				break
			}
		}
	}
	if fallback == nil {
		return nil
	}

	retry := fallback.Clone(r.Context())
	retry.Response = nil
	retry.URL.Path, _ = oauthMetadataRootPath(fallback.URL.Path)
	retry.URL.RawPath = ""
	return retry
}

func shouldTryOAuthMetadataRootFallback(r *http.Request, resp *http.Response) bool {
	_, rootPath, ok := oauthMetadataChainOrigin(r)
	if resp == nil || !ok || rootPath != mcputils.OAuthProtectedResourceMetadataPath {
		return false
	}
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return true
	}
	if !isHTTPRedirect(resp.StatusCode) {
		return false
	}
	location, err := resp.Location()
	return err == nil && !utils.SameHTTPOrigin(r.URL, location)
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

const openIDConfigurationPath = "/.well-known/openid-configuration"

var oauthMetadataPaths = []string{
	mcputils.OAuthProtectedResourceMetadataPath,
	"/.well-known/oauth-authorization-server",
	openIDConfigurationPath,
}

// openIDConfigurationAppendedRequest turns a 404 on
// /.well-known/openid-configuration/<issuer path> into a retry of
// <issuer path>/.well-known/openid-configuration. The MCP spec requires
// clients to try both.
// https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization/authorization-server-discovery#authorization-server-metadata-discovery
func openIDConfigurationAppendedRequest(r *http.Request, resp *http.Response) *http.Request {
	if r.Method != http.MethodGet || resp == nil || resp.StatusCode != http.StatusNotFound {
		return nil
	}
	issuerPath, ok := strings.CutPrefix(r.URL.Path, openIDConfigurationPath+"/")
	if !ok || strings.Trim(issuerPath, "/") == "" {
		return nil
	}
	retry := r.Clone(r.Context())
	retry.Response = nil
	retry.URL.Path = "/" + strings.TrimSuffix(issuerPath, "/") + openIDConfigurationPath
	retry.URL.RawPath = ""
	return retry
}

// oauthMetadataChainOrigin returns the metadata request URL that started a
// redirect chain and its root path, so a redirected metadata response is
// still validated as the document originally requested.
func oauthMetadataChainOrigin(r *http.Request) (*url.URL, string, bool) {
	var origin *url.URL
	var rootPath string
	for candidate, root := range oauthMetadataRequestChain(r) {
		origin, rootPath = candidate.URL, root
	}
	return origin, rootPath, origin != nil
}

// oauthMetadataRequestChain yields metadata requests from the latest request
// back to the request that began the redirect chain.
func oauthMetadataRequestChain(r *http.Request) iter.Seq2[*http.Request, string] {
	return func(yield func(*http.Request, string) bool) {
		for candidate := r; candidate != nil; {
			if root, ok := oauthMetadataRootPath(candidate.URL.Path); ok && !yield(candidate, root) {
				return
			}
			if candidate.Response == nil {
				return
			}
			candidate = candidate.Response.Request
		}
	}
}

// oauthMetadataRootPath returns the well-known path in a metadata request
// path. OpenID's well-known path can also come at the end.
func oauthMetadataRootPath(path string) (string, bool) {
	for _, root := range oauthMetadataPaths {
		if path == root || strings.HasPrefix(path, root+"/") {
			return root, true
		}
	}
	if strings.HasSuffix(path, openIDConfigurationPath) {
		return openIDConfigurationPath, true
	}
	return "", false
}

// oauthMetadataIssuerPath strips the well-known path, so both
// /.well-known/openid-configuration/<issuer path> and
// <issuer path>/.well-known/openid-configuration give <issuer path>.
func oauthMetadataIssuerPath(path, rootPath string) string {
	if issuerPath, ok := strings.CutPrefix(path, rootPath); ok {
		return issuerPath
	}
	return strings.TrimSuffix(path, rootPath)
}
