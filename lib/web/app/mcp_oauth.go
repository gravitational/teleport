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

package app

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/singleflight"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	usercredpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

// mcpOAuthTokenRetryInterval is how long a session waits to ask Auth again when it
// doesn't have a token, otherwise it would call Auth on every request.
const mcpOAuthTokenRetryInterval = 10 * time.Second

type mcpOAuthCredentialClient interface {
	UserExternalCredentialClient() usercredpb.UserExternalCredentialServiceClient
}

// mcpOAuthToken caches a session's access token. We don't use FnCache because its
// TTL is set before the load, and we want to cache until the token's actual expiry.
type mcpOAuthToken struct {
	mu     sync.Mutex
	token  string
	appURI string
	// validUntil is zero if the token doesn't expire.
	validUntil time.Time
	// group keeps a session's concurrent requests to one call to Auth.
	group singleflight.Group
}

func (t *transport) cachedMCPOAuthToken() (token, appURI string, valid bool) {
	cache := &t.mcpOAuthToken
	cache.mu.Lock()
	defer cache.mu.Unlock()
	valid = (cache.token != "" && cache.validUntil.IsZero()) || t.c.clock.Now().Before(cache.validUntil)
	return cache.token, cache.appURI, valid
}

// invalidateMCPOAuthToken holds off the next call to Auth until after the retry
// interval, since Auth can just give us the same token back.
func (t *transport) invalidateMCPOAuthToken() {
	cache := &t.mcpOAuthToken
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.token != "" {
		cache.token, cache.validUntil = "", t.c.clock.Now().Add(mcpOAuthTokenRetryInterval)
	}
}

// setMCPOAuthHeader leaves a header the client sent itself alone, such as with "-H".
func (t *transport) setMCPOAuthHeader(r *http.Request) bool {
	// tokens are stored in this cluster's Auth, and a leaf's App Service shouldn't see them
	if r.Header.Get("Authorization") != "" || t.c.mcpOAuthClient == nil || t.c.isRemoteApp() {
		return false
	}
	token, appURI := t.mcpOAuthAccessToken(r.Context())
	if token == "" {
		return false
	}
	if !t.appServersMatchURI(r.Context(), appURI) {
		// drop it so we pick up a new login after the retry interval
		t.invalidateMCPOAuthToken()
		return false
	}
	r.Header.Set("Authorization", "Bearer "+token)
	return true
}

// appServersMatchURI stops sending a user's token once an app's URI changes. It uses
// resolve because the cached forChecks list can be 10s behind what we dial.
func (t *transport) appServersMatchURI(ctx context.Context, uri string) bool {
	_, servers, err := t.c.appServers.resolve(ctx)
	if err != nil {
		t.c.log.WarnContext(ctx, "Failed to check the app URI of a stored MCP OAuth token", "error", err)
		return false
	}
	for _, server := range servers {
		if server.GetApp().GetURI() != uri {
			t.c.log.WarnContext(ctx, "Not sending a stored MCP OAuth token issued for another app URI, the user must log in again with 'tsh mcp login'", "app", server.GetApp().GetName())
			return false
		}
	}
	return len(servers) > 0
}

func (t *transport) mcpOAuthAccessToken(ctx context.Context) (token, appURI string) {
	if token, appURI, valid := t.cachedMCPOAuthToken(); valid {
		return token, appURI
	}

	done := t.mcpOAuthToken.group.DoChan("", func() (any, error) {
		t.fetchMCPOAuthToken(ctx)
		return nil, nil
	})
	select {
	case <-done:
		token, appURI, _ := t.cachedMCPOAuthToken()
		return token, appURI
	case <-ctx.Done():
		return "", ""
	}
}

// fetchMCPOAuthToken doesn't stop if the request is canceled because other requests
// can be waiting on it.
func (t *transport) fetchMCPOAuthToken(ctx context.Context) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), apidefaults.DefaultIOTimeout)
	defer cancel()

	token, appURI, expires, err := t.requestMCPOAuthToken(ctx)
	switch {
	case trace.IsNotFound(err):
		t.c.log.DebugContext(ctx, "No stored MCP OAuth token", "error", err)
	case err != nil:
		t.c.log.WarnContext(ctx, "Failed to get stored MCP OAuth token", "error", err)
	}

	cache := &t.mcpOAuthToken
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.token, cache.appURI = token, appURI
	now := t.c.clock.Now()
	cache.validUntil = expires
	// drop it when Auth starts refreshing it. If it's already that close to expiring,
	// Auth can't get us a better one, so asking again would only hit the rate limit.
	if refreshAt := expires.Add(-services.MCPOAuthRefreshWindow); !expires.IsZero() && refreshAt.After(now) {
		cache.validUntil = refreshAt
	}
	if cache.token == "" {
		cache.validUntil = now.Add(mcpOAuthTokenRetryInterval)
	}
}

func (t *transport) requestMCPOAuthToken(ctx context.Context) (token, appURI string, expires time.Time, err error) {
	servers, err := t.c.appServers.forChecks(ctx)
	if err != nil {
		return "", "", time.Time{}, trace.Wrap(err)
	}
	if len(servers) == 0 || types.GetMCPServerTransportType(servers[0].GetApp().GetURI()) != types.MCPTransportHTTP {
		return "", "", time.Time{}, trace.NotFound("app is not a streamable HTTP MCP server")
	}
	// an unset mode can still use OAuth, only an explicit not required can't
	if mcp := servers[0].GetApp().GetMCP(); mcp != nil && mcp.Oauth != nil && mcp.Oauth.Mode == types.MCPOAuthModeNotRequired {
		return "", "", time.Time{}, trace.NotFound("app does not use OAuth")
	}
	cert, err := tlsca.ParseCertificatePEM(t.c.ws.GetTLSCert())
	if err != nil {
		return "", "", time.Time{}, trace.Wrap(err)
	}
	resp, err := t.c.mcpOAuthClient.UserExternalCredentialClient().GetMCPOAuthAccessToken(ctx, usercredpb.GetMCPOAuthAccessTokenRequest_builder{
		UserCertificate: cert.Raw,
	}.Build())
	if err != nil {
		return "", "", time.Time{}, trace.Wrap(err)
	}
	if resp.HasExpires() {
		expires = resp.GetExpires().AsTime()
	}
	return resp.GetAccessToken(), resp.GetAppUri(), expires, nil
}
