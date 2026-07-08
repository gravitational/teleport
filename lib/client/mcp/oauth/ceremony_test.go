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
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRunLoginCeremony(t *testing.T) {
	var srvURL string
	mux := http.NewServeMux()
	// RFC 9728 protected resource metadata (path-suffixed form; mcp-go asks
	// for /.well-known/oauth-protected-resource/mcp when the resource is /mcp).
	prHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"resource":              srvURL + "/mcp",
			"authorization_servers": []string{srvURL},
		})
	}
	mux.HandleFunc("/.well-known/oauth-protected-resource", prHandler)
	mux.HandleFunc("/.well-known/oauth-protected-resource/", prHandler)
	// RFC 8414 AS metadata.
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"issuer":                   srvURL,
			"authorization_endpoint":   srvURL + "/authorize",
			"token_endpoint":           srvURL + "/token",
			"registration_endpoint":    srvURL + "/register",
			"response_types_supported": []string{"code"},
		})
	})
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"client_id": "test-client-id"})
	})
	// Auto-approving authorize endpoint: bounce straight back to the loopback.
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("code_challenge") == "" || q.Get("state") == "" || q.Get("client_id") != "test-client-id" {
			http.Error(w, "bad authorize request", http.StatusBadRequest)
			return
		}
		http.Redirect(w, r, q.Get("redirect_uri")+"?code=test-code&state="+url.QueryEscape(q.Get("state")), http.StatusFound)
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.Form.Get("grant_type") != "authorization_code" ||
			r.Form.Get("code") != "test-code" ||
			r.Form.Get("code_verifier") == "" {
			http.Error(w, "bad token request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "ceremony-access",
			"token_type":    "Bearer",
			"refresh_token": "ceremony-refresh",
			"expires_in":    3600,
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	srvURL = srv.URL

	upstream, err := url.Parse(srv.URL + "/mcp")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	creds, err := RunLoginCeremony(ctx, CeremonyConfig{
		UpstreamURL: upstream,
		HTTPClient:  srv.Client(),
		OpenURL: func(authURL string) error {
			// Stand-in for the browser: follow the authorize redirect chain,
			// which lands on the ceremony's loopback callback.
			go func() {
				resp, err := http.Get(authURL)
				if err == nil {
					resp.Body.Close()
				}
			}()
			return nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, "ceremony-access", creds.Token.AccessToken)
	require.Equal(t, "ceremony-refresh", creds.Token.RefreshToken)
	require.False(t, creds.Token.ExpiresAt.IsZero())
	require.Equal(t, "test-client-id", creds.ClientID)
	require.Equal(t, srv.URL+"/token", creds.TokenEndpoint)
	require.Equal(t, srv.URL+"/mcp", creds.Resource)
}

func TestRunLoginCeremonyNoDCR(t *testing.T) {
	var srvURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-protected-resource/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"resource":              srvURL + "/mcp",
			"authorization_servers": []string{srvURL},
		})
	})
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// No registration_endpoint.
		json.NewEncoder(w).Encode(map[string]any{
			"issuer":                   srvURL,
			"authorization_endpoint":   srvURL + "/authorize",
			"token_endpoint":           srvURL + "/token",
			"response_types_supported": []string{"code"},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	srvURL = srv.URL

	upstream, err := url.Parse(srv.URL + "/mcp")
	require.NoError(t, err)
	_, err = RunLoginCeremony(context.Background(), CeremonyConfig{
		UpstreamURL: upstream,
		HTTPClient:  srv.Client(),
		OpenURL:     func(string) error { return nil },
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "pre-registered client")
}

func TestRunLoginCeremonyRejectsInsecureDirectEndpoints(t *testing.T) {
	var srvURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-protected-resource/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"resource":              srvURL + "/mcp",
			"authorization_servers": []string{srvURL},
		})
	})
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"issuer":                   srvURL,
			"authorization_endpoint":   "https://auth.example.com/authorize",
			"token_endpoint":           "http://auth.example.com/token",
			"registration_endpoint":    "https://auth.example.com/register",
			"response_types_supported": []string{"code"},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	srvURL = srv.URL

	upstream, err := url.Parse(srv.URL + "/mcp")
	require.NoError(t, err)
	_, err = RunLoginCeremony(context.Background(), CeremonyConfig{
		UpstreamURL: upstream,
		HTTPClient:  srv.Client(),
		OpenURL:     func(string) error { return nil },
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "must use HTTPS")
}
