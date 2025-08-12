/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package oidc

import (
	"context"
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"github.com/gregjones/httpcache"
	"github.com/zitadel/oidc/v3/pkg/client"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// providerTimeout is the maximum time allowed to fetch provider metadata before
// giving up.
const providerTimeout = 15 * time.Second

// Validate validates an OIDC ID token for the specified claims type.
func ValidateToken[C oidc.Claims](
	ctx context.Context,
	issuerURL string,
	audience string,
	token string,
	opts ...rp.VerifierOption,
) (C, error) {
	return ValidateTokenWithClient[C](
		ctx,
		otelhttp.DefaultClient,
		issuerURL,
		audience,
		token,
		opts...,
	)
}

// ValidateTokenWithClient validates an OIDC token against a generic claim type,
// but accepts a custom HTTP client. This custom client can be used in tandem
// with the client returned by `NewCachingHTTPClient()` to cache OpenID and JWKS
// responses.
func ValidateTokenWithClient[C oidc.Claims](
	ctx context.Context,
	httpClient *http.Client,
	issuerURL string,
	audience string,
	token string,
	opts ...rp.VerifierOption,
) (C, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, providerTimeout)
	defer cancel()

	var nilClaims C

	// TODO(noah): It'd be nice to cache the OIDC discovery document fairly
	// aggressively across join tokens since this isn't going to change very
	// regularly.
	dc, err := client.Discover(timeoutCtx, issuerURL, httpClient)
	if err != nil {
		return nilClaims, trace.Wrap(err, "discovering oidc document")
	}

	// TODO(noah): Ideally we'd cache the remote keyset across joins/join tokens
	// based on the issuer.
	ks := rp.NewRemoteKeySet(httpClient, dc.JwksURI)
	verifier := rp.NewIDTokenVerifier(issuerURL, audience, ks, opts...)
	// TODO(noah): It'd be ideal if we could extend the verifier to use an
	// injected "now" time.

	claims, err := rp.VerifyIDToken[C](timeoutCtx, token, verifier)
	if err != nil {
		return nilClaims, trace.Wrap(err, "verifying token")
	}

	return claims, nil
}

// NewCachingHTTPClient constructs and returns a caching HTTP client. Similar to
// the default client used in `ValidateToken()`, this uses an underlying
// `otelhttp` transport.
// Note that this client should be retained and reused to benefit from caching.
func NewCachingHTTPClient() *http.Client {
	transport := httpcache.NewMemoryCacheTransport()
	transport.Transport = otelhttp.NewTransport(http.DefaultTransport)

	return transport.Client()
}
