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
	"time"

	"github.com/gravitational/trace"
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
) (C, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, providerTimeout)
	defer cancel()

	var nilClaims C

	// TODO(noah): It'd be nice to cache the OIDC discovery document fairly
	// aggressively across join tokens since this isn't going to change very
	// regularly.
	dc, err := client.Discover(timeoutCtx, issuerURL, otelhttp.DefaultClient)
	if err != nil {
		return nilClaims, trace.Wrap(err, "discovering oidc document")
	}

	// TODO(noah): Ideally we'd cache the remote keyset across joins/join tokens
	// based on the issuer.
	ks := rp.NewRemoteKeySet(otelhttp.DefaultClient, dc.JwksURI)
	verifier := rp.NewIDTokenVerifier(issuerURL, audience, ks)
	// TODO(noah): It'd be ideal if we could extend the verifier to use an
	// injected "now" time.

	claims, err := rp.VerifyIDToken[C](timeoutCtx, token, verifier)
	if err != nil {
		return nilClaims, trace.Wrap(err, "verifying token")
	}

	return claims, nil
}

// ValidateTokenNoAuthorizedPartyCheck is a copy of [ValidateToken] with the
// Authorized Party check removed.
//
// As described in https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation,
// the authorized party check is optional. GCP does not set it to a one of the listed audiences
// as expected by this check.
//
// TODO(Joerger): Remove once upstream oidc library makes this check optional.
func ValidateTokenNoAuthorizedPartyCheck[C oidc.Claims](
	ctx context.Context,
	issuerURL string,
	audience string,
	token string,
) (C, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, providerTimeout)
	defer cancel()

	var nilClaims C

	// TODO(noah): It'd be nice to cache the OIDC discovery document fairly
	// aggressively across join tokens since this isn't going to change very
	// regularly.
	dc, err := client.Discover(timeoutCtx, issuerURL, otelhttp.DefaultClient)
	if err != nil {
		return nilClaims, trace.Wrap(err, "discovering oidc document")
	}

	// TODO(noah): Ideally we'd cache the remote keyset across joins/join tokens
	// based on the issuer.
	ks := rp.NewRemoteKeySet(otelhttp.DefaultClient, dc.JwksURI)
	verifier := rp.NewIDTokenVerifier(issuerURL, audience, ks)
	// TODO(noah): It'd be ideal if we could extend the verifier to use an
	// injected "now" time.

	claims, err := verifyIDToken[C](timeoutCtx, token, verifier)
	if err != nil {
		return nilClaims, trace.Wrap(err, "verifying token")
	}

	return claims, nil
}

// verifyIDToken is a copy of [rp.verifyIDToken] with the Authorized Party check removed.
// TODO(Joerger): Remove once upstream oidc library makes this check optional.
func verifyIDToken[C oidc.Claims](ctx context.Context, token string, v *rp.IDTokenVerifier) (claims C, err error) {
	ctx, span := client.Tracer.Start(ctx, "VerifyIDToken")
	defer span.End()

	var nilClaims C

	decrypted, err := oidc.DecryptToken(token)
	if err != nil {
		return nilClaims, err
	}
	payload, err := oidc.ParseToken(decrypted, &claims)
	if err != nil {
		return nilClaims, err
	}

	if err := oidc.CheckSubject(claims); err != nil {
		return nilClaims, err
	}

	if err = oidc.CheckIssuer(claims, v.Issuer); err != nil {
		return nilClaims, err
	}

	if err = oidc.CheckAudience(claims, v.ClientID); err != nil {
		return nilClaims, err
	}

	// Skip Authorized party check.
	// if err = oidc.CheckAuthorizedParty(claims, v.ClientID); err != nil {
	// 	return nilClaims, err
	// }

	if err = oidc.CheckSignature(ctx, decrypted, payload, claims, v.SupportedSignAlgs, v.KeySet); err != nil {
		return nilClaims, err
	}

	if err = oidc.CheckExpiration(claims, v.Offset); err != nil {
		return nilClaims, err
	}

	if err = oidc.CheckIssuedAt(claims, v.MaxAgeIAT, v.Offset); err != nil {
		return nilClaims, err
	}

	if v.Nonce != nil {
		if err = oidc.CheckNonce(claims, v.Nonce(ctx)); err != nil {
			return nilClaims, err
		}
	}

	if err = oidc.CheckAuthorizationContextClassReference(claims, v.ACR); err != nil {
		return nilClaims, err
	}

	if err = oidc.CheckAuthTime(claims, v.MaxAge); err != nil {
		return nilClaims, err
	}
	return claims, nil
}
