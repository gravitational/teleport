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

	"github.com/gravitational/teleport"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, teleport.Component("oidc"))

const (
	// discoveryTTL is the maximum duration a discovery configuration will be
	// cached locally before being discarded
	discoveryTTL = time.Hour

	// keySetTTL is the maximum duration a particular keyset will be allowed to
	// exist before being purged, regardless of whether or not it is being used
	// actively. The underlying library may update its internal cache of keys
	// within this window.
	keySetTTL = time.Hour * 24

	// validatorTTL is a maximum time a particular validator instance should
	// remain in memory before being pruned if left unused.
	validatorTTL = time.Hour * 24 * 2

	// freshnessCheckMinInterval is the minimum period to wait before
	// potentially purging stale validators.
	freshnessCheckMinInterval = time.Hour
)

// validatorKey is a composite key for the validator instance map
type validatorKey struct {
	issuer   string
	audience string
}

// NewCachingTokenValidator creates a caching validator for the given issuer and
// audience, using a real clock.
func NewCachingTokenValidator[C oidc.Claims]() *CachingTokenValidator[C] {
	clock := clockwork.NewRealClock()

	return &CachingTokenValidator[C]{
		clock:      clock,
		instances:  map[validatorKey]*CachingValidatorInstance[C]{},
		lastPruned: clock.Now(),
	}
}

// CachingTokenValidator is a wrapper on top of `CachingValidatorInstance` that
// automatically manages and prunes validator instances for a given
// (issuer, audience) pair. This helps to ensure validators and key sets don't
// remain in memory indefinitely if e.g. a Teleport auth token is modified to
// use a different issuer or removed outright.
type CachingTokenValidator[C oidc.Claims] struct {
	clock clockwork.Clock

	mu         sync.Mutex
	instances  map[validatorKey]*CachingValidatorInstance[C]
	lastPruned time.Time
}

// pruneStaleValidators removes stale validators from the internal validator
// instance map. `v.mu` must already be held.
func (v *CachingTokenValidator[C]) pruneStaleValidators() {
	if v.clock.Since(v.lastPruned) < freshnessCheckMinInterval {
		// Too soon since the last prune, don't bother.
		return
	}

	retained := map[validatorKey]*CachingValidatorInstance[C]{}
	prunedCount := 0
	for k, v := range v.instances {
		if v.IsStale() {
			prunedCount += 1
			continue
		}

		retained[k] = v
	}

	v.instances = retained
	v.lastPruned = v.clock.Now()
	log.DebugContext(context.Background(), "Pruned stale OIDC validators", "count", prunedCount)
}

// GetValidator retreives a validator for the given issuer and audience. This
// will create a new validator instance if necessary, and will occasionally
// prune old instances that have not been used to validate any tokens in some
// time.
func (v *CachingTokenValidator[C]) GetValidator(issuer, audience string) *CachingValidatorInstance[C] {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.pruneStaleValidators()

	key := validatorKey{issuer: issuer, audience: audience}
	if validator, ok := v.instances[key]; ok {
		return validator
	}

	validator := &CachingValidatorInstance[C]{
		clock:            v.clock,
		issuer:           issuer,
		audience:         audience,
		validatorExpires: v.clock.Now().Add(validatorTTL),
		verifierFn:       zoidcTokenVerifier[C],
	}

	v.instances[key] = validator

	return validator
}

// CachingValidatorInstance provides an issuer-specific cache. It separately
// caches the discovery config and `oidc.KeySet` to ensure each is reasonably
// fresh, and purges sufficiently old key sets to ensure old keys are not
// retained indefinitely.
type CachingValidatorInstance[C oidc.Claims] struct {
	issuer   string
	audience string
	clock    clockwork.Clock

	mu                     sync.Mutex
	discoveryConfig        *oidc.DiscoveryConfiguration
	discoveryConfigExpires time.Time
	lastJWKSURI            string
	keySet                 oidc.KeySet
	keySetExpires          time.Time

	validatorExpires time.Time

	// verifierFn is the function that actually verifies the token using the
	// oidc library. `zitadel/oidc` doesn't provide any way to override the
	// clock, so we use this for tests.
	verifierFn func(
		ctx context.Context,
		issuer,
		clientID string,
		keySet oidc.KeySet,
		token string,
		opts ...rp.VerifierOption,
	) (C, error)
}

func (v *CachingValidatorInstance[C]) getKeySet(
	ctx context.Context,
) (oidc.KeySet, error) {
	// Note: We could consider an RWLock or singleflight if perf proves to be
	// poor here. As written, I don't expect serialized warm-cache requests to
	// accumulate enough to be worth the added complexity.
	v.mu.Lock()
	defer v.mu.Unlock()

	l := log.With("issuer", v.issuer, "audience", v.audience)
	now := v.clock.Now()

	// Mark this validator as fresh.
	v.validatorExpires = now.Add(validatorTTL)

	if !v.discoveryConfigExpires.IsZero() && now.After(v.discoveryConfigExpires) {
		// Invalidate the cached value.
		v.discoveryConfig = nil
		v.discoveryConfigExpires = time.Time{}

		l.DebugContext(ctx, "Invalidating expired discovery config")
	}

	if v.discoveryConfig == nil {
		l.DebugContext(ctx, "Fetching new discovery config")

		// Note: This is the only blocking call inside the mutex.
		// In the future, it might be a good idea to fetch the new discovery
		// config async and keep it available if the refresh fails.
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

	// If this upstream issue is fixed, we can remove this in favor of keeping
	// the KeySet: https://github.com/zitadel/oidc/issues/747
	if !v.keySetExpires.IsZero() && now.After(v.keySetExpires) {
		// Invalidate the cached value.
		v.keySet = nil
		v.keySetExpires = time.Time{}

		l.DebugContext(ctx, "Invalidating expired KeySet")
	}

	if v.keySet == nil {
		l.DebugContext(ctx, "Creating new remote KeySet")
		v.keySet = rp.NewRemoteKeySet(newHTTPClient(), v.discoveryConfig.JwksURI)
		v.keySetExpires = now.Add(keySetTTL)
	}

	return v.keySet, nil
}

func zoidcTokenVerifier[C oidc.Claims](
	ctx context.Context,
	issuer,
	clientID string,
	keySet oidc.KeySet,
	token string,
	opts ...rp.VerifierOption,
) (C, error) {
	var nilClaims C
	verifier := rp.NewIDTokenVerifier(issuer, clientID, keySet, opts...)

	// Note: VerifyIDToken() may mutate the KeySet (if the keyset is empty or if
	// it encounters an unknown `kid`). The keyset manages a mutex of its own,
	// so we don't need to protect this operation. It's acceptable for this
	// keyset to be swapped in another thread and still used here; it will just
	// be GC'd afterward.
	claims, err := rp.VerifyIDToken[C](ctx, token, verifier)
	if err != nil {
		return nilClaims, trace.Wrap(err, "verifying token")
	}

	return claims, nil
}

// ValidateToken verifies a compact encoded token against the configured
// issuer and keys, potentially using cached OpenID configuration and JWKS
// values.
func (v *CachingValidatorInstance[C]) ValidateToken(
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

	claims, err := v.verifierFn(ctx, v.issuer, v.audience, ks, token, opts...)
	if err != nil {
		return nilClaims, trace.Wrap(err)
	}

	return claims, nil
}

// Expires returns the time at which this validator will expire if left unused.
func (v *CachingValidatorInstance[C]) Expires() time.Time {
	v.mu.Lock()
	defer v.mu.Unlock()

	return v.validatorExpires
}

// IsStale returns true if this validator has not been asked to validate any
// tokens for at least `validatorTTL` and is eligible to be pruned.
func (v *CachingValidatorInstance[C]) IsStale() bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	return v.clock.Now().After(v.validatorExpires)
}

func newHTTPClient() *http.Client {
	return otelhttp.DefaultClient
}
