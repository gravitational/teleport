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

package credentials

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/gravitational/trace"
	"github.com/zitadel/oidc/v3/pkg/client"

	"github.com/gravitational/teleport/api/types"
)

// discoverTimeout is the timeout for an OIDC discovery request.
// The request times out, rather than retries, because some of the endpoints to be discovered are optional,
// and we don't want to make the user wait indefinitely if they can't be discovered.
const discoverTimeout = 15 * time.Second

type OAuthEndpointKind int

const (
	OAuthEndpointKindUnspecified OAuthEndpointKind = iota
	OAuthEndpointKindAuthorize
	OAuthEndpointKindToken
)

// OAuthEndpoints are the resolved authorize and token endpoints for an OAuth Proxy integration.
type OAuthEndpoints struct {
	Authorize, Token string
}

// has returns true if all kinds are present.
func (e *OAuthEndpoints) has(kinds ...OAuthEndpointKind) bool {
	for _, kind := range kinds {
		switch kind {
		case OAuthEndpointKindAuthorize:
			if e.Authorize == "" {
				return false
			}
		case OAuthEndpointKindToken:
			if e.Token == "" {
				return false
			}
		default:
			return false
		}
	}

	return true
}

// ResolveOAuthProxyEndpoints resolves the authorization and token endpoints for an OAuth Proxy integration.
// If the endpoints are not present on the integration spec, then OIDC discovery is attempted using the issuer present on the spec.
func ResolveOAuthProxyEndpoints(ctx context.Context, ig types.Integration, httpClient *http.Client, kinds ...OAuthEndpointKind) (*OAuthEndpoints, error) {
	spec := ig.GetOAuthProxyIntegrationSpec()
	if spec == nil {
		return nil, trace.BadParameter("not an OAuth Proxy integration")
	}

	if len(kinds) == 0 {
		return nil, trace.BadParameter("at least one endpoint kind is required")
	}

	resolved := &OAuthEndpoints{
		Authorize: spec.AuthorizationEndpoint,
		Token:     spec.TokenEndpoint,
	}

	if resolved.has(kinds...) {
		return resolved, nil
	}

	discoverCtx, cancel := context.WithTimeout(ctx, discoverTimeout)
	defer cancel()

	// TODO(nixpig): Use ssrf-safe client (next PR in stack) before exposing on service.
	dc, err := client.Discover(discoverCtx, spec.IssuerUrl, httpClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if resolved.Authorize == "" && dc.AuthorizationEndpoint != "" {
		if err := ValidateHTTPSEndpoint(dc.AuthorizationEndpoint); err != nil {
			return nil, trace.Wrap(err)
		}
		resolved.Authorize = dc.AuthorizationEndpoint
	}
	if resolved.Token == "" && dc.TokenEndpoint != "" {
		if err := ValidateHTTPSEndpoint(dc.TokenEndpoint); err != nil {
			return nil, trace.Wrap(err)
		}
		resolved.Token = dc.TokenEndpoint
	}

	return resolved, nil
}

// ValidateHTTPSEndpoint ensures the given endpoint is a valid HTTPS URL.
func ValidateHTTPSEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	switch {
	case err != nil:
		return trace.Wrap(err)
	case u.Scheme != "https" || u.Host == "":
		return trace.BadParameter("not a valid HTTPS URL")
	}

	return nil
}

// GetOAuthProxyClient returns the client ID and client secret for an OAuth Proxy integration from its static credentials ref.
func GetOAuthProxyClient(ctx context.Context, ig types.Integration, getter ByLabelsGetter) (string, string, error) {
	creds := ig.GetCredentials()
	if creds == nil || creds.GetStaticCredentialsRef() == nil {
		return "", "", trace.NotFound("integration %q has no credentials", ig.GetName())
	}

	cred, err := GetByPurpose(ctx, creds.GetStaticCredentialsRef(), PurposeOAuthProxyClient, getter)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	clientID, clientSecret := cred.GetOAuthClientSecret()

	return clientID, clientSecret, nil
}
