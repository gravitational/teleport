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

package circleci

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/zitadel/oidc/v3/pkg/client"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func ValidateToken(
	ctx context.Context,
	issuerURLTemplate string,
	organizationID string,
	token string,
) (*IDTokenClaims, error) {
	issuer := issuerURL(issuerURLTemplate, organizationID)

	// TODO(noah): It'd be nice to cache the OIDC discovery document fairly
	// aggressively across join tokens since this isn't going to change very
	// regularly.
	dc, err := client.Discover(ctx, issuer, otelhttp.DefaultClient)
	if err != nil {
		return nil, trace.Wrap(err, "discovering oidc document")
	}

	// TODO(noah): Ideally we'd cache the remote keyset across joins/join tokens
	// based on the issuer.
	ks := rp.NewRemoteKeySet(otelhttp.DefaultClient, dc.JwksURI)
	verifier := rp.NewIDTokenVerifier(issuer, organizationID, ks)
	// TODO(noah): It'd be ideal if we could extend the verifier to use an
	// injected "now" time.

	claims, err := rp.VerifyIDToken[*IDTokenClaims](ctx, token, verifier)
	if err != nil {
		return nil, trace.Wrap(err, "verifying token")
	}

	return claims, nil
}
