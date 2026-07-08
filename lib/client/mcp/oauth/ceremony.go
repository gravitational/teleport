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
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"
	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"
)

// CeremonyConfig configures the interactive OAuth login ceremony.
type CeremonyConfig struct {
	// UpstreamURL is the MCP app's upstream URL (see UpstreamURL).
	UpstreamURL *url.URL
	// HTTPClient must route app-host requests through the Teleport tunnel
	// and all other requests directly (see NewHTTPClient).
	HTTPClient *http.Client
	// OpenURL is called once with the authorization URL the user must visit.
	// The caller decides how to present it (open a browser, print it).
	OpenURL func(authURL string) error
	// ClientName is the client_name sent during dynamic client registration.
	// Defaults to "Teleport tsh".
	ClientName string
}

func (cfg *CeremonyConfig) checkAndSetDefaults() error {
	if cfg.UpstreamURL == nil {
		return trace.BadParameter("missing UpstreamURL")
	}
	if cfg.HTTPClient == nil {
		return trace.BadParameter("missing HTTPClient")
	}
	if cfg.OpenURL == nil {
		return trace.BadParameter("missing OpenURL")
	}
	if cfg.ClientName == "" {
		cfg.ClientName = "Teleport tsh"
	}
	return nil
}

// RunLoginCeremony runs the interactive OAuth flow for an HTTP MCP app:
// RFC 9728/8414 discovery, dynamic client registration, PKCE authorization
// code flow with a loopback redirect, and token exchange. It returns the
// resulting credentials; persisting them is the caller's job.
func RunLoginCeremony(ctx context.Context, cfg CeremonyConfig) (*Credentials, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer listener.Close()
	redirectURI := fmt.Sprintf("http://%s/callback", listener.Addr().String())

	memStore := mcpclienttransport.NewMemoryTokenStore()
	handler := mcpclienttransport.NewOAuthHandler(mcpclienttransport.OAuthConfig{
		RedirectURI: redirectURI,
		PKCEEnabled: true,
		TokenStore:  memStore,
		HTTPClient:  cfg.HTTPClient,
	})
	handler.SetBaseURL(cfg.UpstreamURL.String())

	metadata, err := handler.GetServerMetadata(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "discovering OAuth authorization server for %s", cfg.UpstreamURL)
	}
	if metadata.RegistrationEndpoint == "" {
		return nil, trace.NotImplemented("this provider requires a pre-registered client (not yet supported): the authorization server does not offer dynamic client registration")
	}
	if err := validateMetadataEndpoints(metadata); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := handler.RegisterClient(ctx, cfg.ClientName); err != nil {
		return nil, trace.Wrap(err, "registering OAuth client")
	}

	codeVerifier, err := mcpclienttransport.GenerateCodeVerifier()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	codeChallenge := mcpclienttransport.GenerateCodeChallenge(codeVerifier)
	state, err := mcpclienttransport.GenerateState()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authURL, err := handler.GetAuthorizationURL(ctx, state, codeChallenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	type callbackResult struct {
		code, state string
		err         error
	}
	resultCh := make(chan callbackResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		result := callbackResult{code: q.Get("code"), state: q.Get("state")}
		if errCode := q.Get("error"); errCode != "" {
			result.err = trace.AccessDenied("authorization failed: %s %s", errCode, q.Get("error_description"))
			http.Error(w, "Authorization failed. You can close this window and return to the terminal.", http.StatusBadRequest)
		} else {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, "<html><body><p>Login successful. You can close this window and return to the terminal.</p></body></html>")
		}
		select {
		case resultCh <- result:
		default:
		}
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(listener)
	defer srv.Close()

	if err := cfg.OpenURL(authURL); err != nil {
		return nil, trace.Wrap(err)
	}

	var res callbackResult
	select {
	case res = <-resultCh:
	case <-ctx.Done():
		return nil, trace.Wrap(ctx.Err(), "waiting for OAuth authorization callback")
	}
	if res.err != nil {
		return nil, trace.Wrap(res.err)
	}
	if err := handler.ProcessAuthorizationResponse(ctx, res.code, res.state, codeVerifier); err != nil {
		return nil, trace.Wrap(err, "exchanging authorization code")
	}

	token, err := memStore.GetToken(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Credentials{
		Token:         token,
		ClientID:      handler.GetClientID(),
		ClientSecret:  handler.GetClientSecret(),
		TokenEndpoint: metadata.TokenEndpoint,
		Resource:      fetchResourceID(ctx, cfg.HTTPClient, cfg.UpstreamURL),
		UpstreamURL:   cfg.UpstreamURL.String(),
		Issuer:        metadata.Issuer,
	}, nil
}

func validateMetadataEndpoints(metadata *mcpclienttransport.AuthServerMetadata) error {
	if err := validateOAuthEndpoint(metadata.AuthorizationEndpoint, "authorization_endpoint"); err != nil {
		return trace.Wrap(err)
	}
	if err := validateOAuthEndpoint(metadata.TokenEndpoint, "token_endpoint"); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(validateOAuthEndpoint(metadata.RegistrationEndpoint, "registration_endpoint"))
}

// fetchResourceID returns the RFC 8707 resource indicator from the protected
// resource metadata, falling back to the upstream URL itself. Best-effort:
// mcp-go discovered this value already but does not expose it, and refreshes
// want to keep sending it.
func fetchResourceID(ctx context.Context, httpClient *http.Client, upstream *url.URL) string {
	fallback := upstream.String()
	for _, path := range []string{
		"/.well-known/oauth-protected-resource" + strings.TrimSuffix(upstream.EscapedPath(), "/"),
		"/.well-known/oauth-protected-resource",
	} {
		wellKnown := *upstream
		wellKnown.Path = path
		wellKnown.RawPath, wellKnown.RawQuery, wellKnown.Fragment = "", "", ""

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnown.String(), nil)
		if err != nil {
			continue
		}
		req.Header.Set("Accept", "application/json")
		resp, err := httpClient.Do(req)
		if err != nil {
			continue
		}
		var pr struct {
			Resource string `json:"resource"`
		}
		if resp.StatusCode == http.StatusOK {
			err = json.NewDecoder(resp.Body).Decode(&pr)
		}
		resp.Body.Close()
		if err == nil && pr.Resource != "" {
			return pr.Resource
		}
	}
	return fallback
}
