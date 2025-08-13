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

package oidc

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/zitadel/oidc/v3/pkg/client"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const (
	// discoveryTTL is the maximum duration a discovery configuration will be
	// cached locally before being discarded
	discoveryTTL = time.Hour

	// keySetTTL is the maximum duration a particular keyset will be allowed to
	// exist, before being purged. The underlying library may update its
	// internal cache of keys within this window,
	keySetTTL = time.Hour * 24
)

// CachingTokenValidator provides an issuer-specific cache
type CachingTokenValidator[C oidc.Claims] struct {
	issuer   string
	audience string
	clock    clockwork.Clock

	mu                     sync.Mutex
	discoveryConfig        *oidc.DiscoveryConfiguration
	discoveryConfigExpires time.Time
	lastJWKSURI string
	keySet                 oidc.KeySet
	keySetExpires          time.Time
}

func NewCachingTokenValidator[C oidc.Claims](issuer, audience string) *CachingTokenValidator[C] {
	return &CachingTokenValidator[C]{
		issuer:   issuer,
		audience: audience,
		clock:    clockwork.NewRealClock(),
	}
}

func (v *CachingTokenValidator[C]) getKeySet(
	ctx context.Context,
) (oidc.KeySet, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := v.clock.Now()
	if !v.discoveryConfigExpires.IsZero() && now.After(v.discoveryConfigExpires) {
		// Invalidate the cached value.
		v.discoveryConfig = nil
		v.discoveryConfigExpires = time.Time{}
	}

	if v.discoveryConfig == nil {
		// Note: This is the only blocking call inside the mutex.
		dc, err := client.Discover(ctx, v.issuer, newHTTPClient())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		v.discoveryConfig = dc
		v.discoveryConfigExpires = now.Add(discoveryTTL)

		if v.lastJWKSURI != "" && v.lastJWKSURI != dc.JwksURI {
			// If the JWKS URI has changed, expire the keyset now.
			v.keySet = nil
			v.keySetExpires = time.Time{}
		}
		v.lastJWKSURI = dc.JwksURI
	}

	if !v.keySetExpires.IsZero() && now.After(v.keySetExpires) {
		// Invalidate the cached value.
		v.keySet = nil
		v.keySetExpires = time.Time{}
	}

	if v.keySet == nil {
		v.keySet = rp.NewRemoteKeySet(newHTTPClient(), v.discoveryConfig.JwksURI)
		v.keySetExpires = now.Add(keySetTTL)
	}

	return v.keySet, nil
}

func (v *CachingTokenValidator[C]) ValidateToken(
	ctx context.Context,
	token string,
	opts ...rp.VerifierOption,
) (C, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, providerTimeout)
	defer cancel()

	var nilClaims C

	ks, err := v.getKeySet(timeoutCtx)
	if err != nil {
		return nilClaims, trace.Wrap(err)
	}
	verifier := rp.NewIDTokenVerifier(v.issuer, v.audience, ks, opts...)

	// Note: VerifyIDToken() may mutate the KeySet (if the keyset is empty or if
	// it encounters an unknown `kid`). The keyset manages a mutex of its own,
	// so we don't need to protect this operation.
	claims, err := rp.VerifyIDToken[C](timeoutCtx, token, verifier)
	if err != nil {
		return nilClaims, trace.Wrap(err, "verifying token")
	}

	return claims, nil
}

func newHTTPClient() *http.Client {
	return otelhttp.DefaultClient
}
