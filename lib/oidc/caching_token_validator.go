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
	"log/slog"
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
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
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
)

// ValidatorKey is a caching key for validator instances. Keys at minimum must
// contain an issuer an audience (required to construct validators) but may also
// contain other caching keys as desired.
type ValidatorKey interface {
	comparable

	GetIssuer() string
	GetAudience() string
}

// StandardValidatorKey is a composite key for the validator instance map
type StandardValidatorKey struct {
	issuer   string
	audience string
}

func (k StandardValidatorKey) GetIssuer() string {
	return k.issuer
}

func (k StandardValidatorKey) GetAudience() string {
	return k.audience
}

func NewStandardValidatorKey(issuer, audience string) StandardValidatorKey {
	return StandardValidatorKey{
		issuer:   issuer,
		audience: audience,
	}
}

// NewCachingTokenValidator creates a caching validator for the given issuer and
// audience, using a real clock.
func NewCachingTokenValidator[C oidc.Claims, K ValidatorKey](clock clockwork.Clock) (*CachingTokenValidator[C, K], error) {
	if clock == nil {
		clock = clockwork.NewRealClock()
	}

	cache, err := utils.NewFnCache(utils.FnCacheConfig{
		Clock:       clock,
		TTL:         validatorTTL,
		ReloadOnErr: true,
	})
	if err != nil {
		return nil, err
	}

	return &CachingTokenValidator[C, K]{
		clock: clock,
		cache: cache,
	}, nil
}

// CachingTokenValidator is a wrapper on top of `CachingValidatorInstance` that
// automatically manages and prunes validator instances for a given
// (issuer, audience) pair. This helps to ensure validators and key sets don't
// remain in memory indefinitely if e.g. a Teleport auth token is modified to
// use a different issuer or removed outright.
type CachingTokenValidator[C oidc.Claims, K ValidatorKey] struct {
	clock clockwork.Clock

	cache *utils.FnCache
}

// ClientMutator is used to modify settings on the constructed http client.
// Note that if your downstream use case contains user-configurable client
// options (e.g. custom CA, insecure verification, timeout, etc) you MUST
// include those parameters as components of a custom caching key or changes to
// those parameters will not take effect until the cache is busted (min 48 hours
// or a change to something in the key). It is okay to hash values (e.g. CA
// PEM) or similar.
type ClientMutator func(client *http.Client) error

// GetValidatorWithKey returns a caching token validator using a custom caching
// key. See the ClientMutator docstring for notes about using client mutators
// with caching.
func (v *CachingTokenValidator[C, K]) GetValidatorWithKey(
	ctx context.Context,
	key K,
	opts ...ClientMutator,
) (*CachingValidatorInstance[C], error) {
	instance, err := utils.FnCacheGet(ctx, v.cache, key, func(ctx context.Context) (*CachingValidatorInstance[C], error) {
		transport, err := defaults.Transport()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		client := &http.Client{
			Transport: NewOIDCRoundTripper(otelhttp.NewTransport(transport)),
		}

		for _, mutator := range opts {
			if err := mutator(client); err != nil {
				return nil, trace.Wrap(err, "constructing cached http client")
			}
		}

		return &CachingValidatorInstance[C]{
			client:     client,
			clock:      v.clock,
			issuer:     key.GetIssuer(),
			audience:   key.GetAudience(),
			verifierFn: zoidcTokenVerifier[C],
			logger:     log.With("issuer", key.GetIssuer(), "audience", key.GetAudience()),
		}, nil
	})

	return instance, err
}

// CachingValidatorInstance provides an issuer-specific cache. It separately
// caches the discovery config and `oidc.KeySet` to ensure each is reasonably
// fresh, and purges sufficiently old key sets to ensure old keys are not
// retained indefinitely.
type CachingValidatorInstance[C oidc.Claims] struct {
	issuer   string
	audience string
	clock    clockwork.Clock
	client   *http.Client
	logger   *slog.Logger

	mu                     sync.Mutex
	discoveryConfig        *oidc.DiscoveryConfiguration
	discoveryConfigExpires time.Time
	lastJWKSURI            string
	keySet                 oidc.KeySet
	keySetExpires          time.Time

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

	now := v.clock.Now()

	if !v.discoveryConfigExpires.IsZero() && now.After(v.discoveryConfigExpires) {
		// Invalidate the cached value.
		v.discoveryConfig = nil
		v.discoveryConfigExpires = time.Time{}

		v.logger.DebugContext(ctx, "Invalidating expired discovery config")
	}

	if v.discoveryConfig == nil {
		v.logger.DebugContext(ctx, "Fetching new discovery config")

		// Note: This is the only blocking call inside the mutex.
		// In the future, it might be a good idea to fetch the new discovery
		// config async and keep it available if the refresh fails.
		dc, err := client.Discover(ctx, v.issuer, v.client)
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

		v.logger.DebugContext(ctx, "Invalidating expired KeySet")
	}

	if v.keySet == nil {
		v.logger.DebugContext(ctx, "Creating new remote KeySet")
		v.keySet = rp.NewRemoteKeySet(v.client, v.discoveryConfig.JwksURI)
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
	verifier := rp.NewIDTokenVerifier(issuer, clientID, keySet, opts...)

	// Note: VerifyIDToken() may mutate the KeySet (if the keyset is empty or if
	// it encounters an unknown `kid`). The keyset manages a mutex of its own,
	// so we don't need to protect this operation. It's acceptable for this
	// keyset to be swapped in another thread and still used here; it will just
	// be GC'd afterward.
	claims, err := rp.VerifyIDToken[C](ctx, token, verifier)
	if err != nil {
		return *new(C), trace.Wrap(err, "verifying token")
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

	ks, err := v.getKeySet(timeoutCtx)
	if err != nil {
		return *new(C), trace.Wrap(err)
	}

	claims, err := v.verifierFn(ctx, v.issuer, v.audience, ks, token, opts...)
	if err != nil {
		return *new(C), trace.Wrap(err)
	}

	return claims, nil
}
