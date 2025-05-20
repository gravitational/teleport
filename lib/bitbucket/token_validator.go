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

package bitbucket

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/zitadel/oidc/v3/pkg/client"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// providerTimeout is the maximum time allowed to fetch provider metadata before
// giving up.
const providerTimeout = 15 * time.Second

// IDTokenValidator validates a Bitbucket issued ID Token.
type IDTokenValidator struct{}

// NewIDTokenValidator returns an initialized IDTokenValidator
func NewIDTokenValidator() *IDTokenValidator {
	return &IDTokenValidator{}
}

// Validate validates a Bitbucket issued ID token.
func (id *IDTokenValidator) Validate(
	ctx context.Context, issuerURL, audience, token string,
) (*IDTokenClaims, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, providerTimeout)
	defer cancel()

	// TODO(noah): It'd be nice to cache the OIDC discovery document fairly
	// aggressively across join tokens since this isn't going to change very
	// regularly.
	dc, err := client.Discover(timeoutCtx, issuerURL, otelhttp.DefaultClient)
	if err != nil {
		return nil, trace.Wrap(err, "discovering oidc document")
	}

	// TODO(noah): Ideally we'd cache the remote keyset across joins/join tokens
	// based on the issuer.
	ks := rp.NewRemoteKeySet(otelhttp.DefaultClient, dc.JwksURI)
	verifier := rp.NewIDTokenVerifier(issuerURL, audience, ks)
	// TODO(noah): It'd be ideal if we could extend the verifier to use an
	// injected "now" time.

	claims, err := rp.VerifyIDToken[*IDTokenClaims](timeoutCtx, token, verifier)
	if err != nil {
		return nil, trace.Wrap(err, "verifying token")
	}

	return claims, nil
}
