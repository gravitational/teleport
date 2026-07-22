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
	"crypto"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/gravitational/teleport/lib/cryptosuites"
)

// fakeIDP provides a minimal fake OIDC provider for use in tests
type fakeIDP struct {
	t         *testing.T
	clock     *clockwork.FakeClock
	signer    jose.Signer
	publicKey crypto.PublicKey
	server    *httptest.Server
	audience  string

	useAlternateJWKSEndpoint atomic.Bool
	configRequests           atomic.Uint32
	jwksRequests             atomic.Uint32
}

func newFakeIDP(t *testing.T, clock *clockwork.FakeClock, audience string) *fakeIDP {
	privateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.RSA2048)
	require.NoError(t, err)

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: privateKey},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	require.NoError(t, err)

	f := &fakeIDP{
		clock:     clock,
		signer:    signer,
		publicKey: privateKey.Public(),
		t:         t,
		audience:  audience,
	}

	providerMux := http.NewServeMux()
	providerMux.HandleFunc(
		"/.well-known/openid-configuration",
		f.handleOpenIDConfig,
	)
	providerMux.HandleFunc(
		"/.well-known/jwks",
		f.handleJWKSEndpoint,
	)
	providerMux.HandleFunc(
		"/.well-known/jwks-alt",
		f.handleJWKSEndpoint,
	)

	srv := httptest.NewServer(providerMux)
	t.Cleanup(srv.Close)
	f.server = srv
	return f
}

func (f *fakeIDP) issuer() string {
	return f.server.URL
}

func (f *fakeIDP) handleOpenIDConfig(w http.ResponseWriter, r *http.Request) {
	jwksURI := f.issuer() + "/.well-known/jwks"
	if f.useAlternateJWKSEndpoint.Load() {
		jwksURI += "-alt"
	}

	response := map[string]any{
		"claims_supported": []string{
			"sub",
			"iss",
		},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"issuer":                                f.issuer(),
		"jwks_uri":                              jwksURI,
		"response_types_supported":              []string{"id_token"},
		"scopes_supported":                      []string{"openid"},
		"subject_types_supported":               []string{"public"},
	}
	responseBytes, err := json.Marshal(response)
	require.NoError(f.t, err)
	_, err = w.Write(responseBytes)
	require.NoError(f.t, err)

	f.configRequests.Add(1)
}

func (f *fakeIDP) handleJWKSEndpoint(w http.ResponseWriter, r *http.Request) {
	jwks := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Key: f.publicKey,
			},
		},
	}
	responseBytes, err := json.Marshal(jwks)
	require.NoError(f.t, err)
	_, err = w.Write(responseBytes)
	require.NoError(f.t, err)

	f.jwksRequests.Add(1)
}

func (f *fakeIDP) issueToken(
	t *testing.T,
	audience,
	sub string,
	ttl time.Duration,
) string {
	claims := oidc.TokenClaims{
		Issuer:     f.issuer(),
		Subject:    sub,
		Audience:   oidc.Audience{audience},
		IssuedAt:   oidc.FromTime(f.clock.Now()),
		NotBefore:  oidc.FromTime(f.clock.Now()),
		Expiration: oidc.FromTime(f.clock.Now().Add(ttl)),
	}

	token, err := jwt.Signed(f.signer).
		Claims(claims).
		Serialize()
	require.NoError(t, err)

	return token
}

// TestCachingTokenValidator runs various tests against the caching token
// validator
func TestCachingTokenValidator(t *testing.T) {
	t.Parallel()

	const defaultAudience = "example.teleport.sh"

	// A minimal validator that skips most checks, especially any that depend on
	// the system clock. We do this mainly to still invoke
	// `keySet.VerifySignature()`.
	minimalValidator := func() func(
		context.Context,
		string, string, oidc.KeySet, string, ...rp.VerifierOption) (*oidc.TokenClaims, error) {
		return func(
			ctx context.Context,
			issuer,
			clientID string,
			keySet oidc.KeySet,
			token string,
			opts ...rp.VerifierOption,
		) (*oidc.TokenClaims, error) {
			var claims oidc.TokenClaims
			_, err := oidc.ParseToken(token, &claims)
			if err != nil {
				return nil, err
			}

			jws, err := jose.ParseSigned(token, []jose.SignatureAlgorithm{jose.RS256})
			if err != nil {
				return nil, err
			}

			_, err = keySet.VerifySignature(ctx, jws)
			if err != nil {
				return nil, err
			}

			return &claims, nil
		}
	}

	tests := []struct {
		name     string
		audience string
		execute  func(t *testing.T, idp *fakeIDP, v *CachingTokenValidator[*oidc.TokenClaims, StandardValidatorKey])
	}{
		{
			name:     "empty",
			audience: defaultAudience,
			execute: func(t *testing.T, idp *fakeIDP, v *CachingTokenValidator[*oidc.TokenClaims, StandardValidatorKey]) {
				// Do nothing.
				require.Zero(t, idp.configRequests.Load())
				require.Zero(t, idp.jwksRequests.Load())
			},
		},
		{
			name:     "single validator",
			audience: defaultAudience,
			execute: func(t *testing.T, idp *fakeIDP, v *CachingTokenValidator[*oidc.TokenClaims, StandardValidatorKey]) {
				val, err := v.GetValidatorWithKey(t.Context(), NewStandardValidatorKey(idp.issuer(), defaultAudience))
				require.NoError(t, err)

				token := idp.issueToken(t, defaultAudience, "a", time.Hour)
				claims, err := val.ValidateToken(t.Context(), token)
				require.NoError(t, err)

				require.Equal(t, "a", claims.Subject)
				require.EqualValues(t, 1, idp.configRequests.Load())
				require.EqualValues(t, 1, idp.jwksRequests.Load())

				token = idp.issueToken(t, defaultAudience, "b", time.Hour)
				claims, err = val.ValidateToken(t.Context(), token)
				require.NoError(t, err)

				require.Equal(t, "b", claims.Subject)
				require.EqualValues(t, 1, idp.configRequests.Load())
				require.EqualValues(t, 1, idp.jwksRequests.Load())
			},
		},
		{
			name:     "multiple validators",
			audience: defaultAudience,
			execute: func(t *testing.T, idp *fakeIDP, v *CachingTokenValidator[*oidc.TokenClaims, StandardValidatorKey]) {
				v1, err := v.GetValidatorWithKey(t.Context(), NewStandardValidatorKey(idp.issuer(), "a.teleport.sh"))
				require.NoError(t, err)
				v2, err := v.GetValidatorWithKey(t.Context(), NewStandardValidatorKey(idp.issuer(), "b.teleport.sh"))
				require.NoError(t, err)

				token := idp.issueToken(t, "a.teleport.sh", "a", time.Hour)
				claims, err := v1.ValidateToken(t.Context(), token)
				require.NoError(t, err)

				require.Equal(t, "a", claims.Subject)
				require.EqualValues(t, 1, idp.configRequests.Load(), "config")
				require.EqualValues(t, 1, idp.jwksRequests.Load(), "jwks")

				token = idp.issueToken(t, "b.teleport.sh", "b", time.Hour)
				claims, err = v2.ValidateToken(t.Context(), token)
				require.NoError(t, err)

				require.Equal(t, "b", claims.Subject)
				require.EqualValues(t, 2, idp.configRequests.Load())
				require.EqualValues(t, 2, idp.jwksRequests.Load())

				// Validating against a bad token should fail, and should not
				// result in spurious requests.
				token = idp.issueToken(t, "c.teleport.sh", "c", time.Hour)
				_, err = v2.ValidateToken(t.Context(), token)
				require.Error(t, err)

				require.EqualValues(t, 2, idp.configRequests.Load())
				require.EqualValues(t, 2, idp.jwksRequests.Load())
			},
		},
		{
			name:     "expired config",
			audience: defaultAudience,
			execute: func(t *testing.T, idp *fakeIDP, v *CachingTokenValidator[*oidc.TokenClaims, StandardValidatorKey]) {
				val, err := v.GetValidatorWithKey(t.Context(), NewStandardValidatorKey(idp.issuer(), defaultAudience))
				require.NoError(t, err)
				val.verifierFn = minimalValidator()

				token := idp.issueToken(t, defaultAudience, "a", time.Hour)
				_, err = val.ValidateToken(t.Context(), token)
				require.NoError(t, err)

				require.EqualValues(t, 1, idp.configRequests.Load())
				require.EqualValues(t, 1, idp.jwksRequests.Load())

				idp.clock.Advance(discoveryTTL + time.Minute)
				token = idp.issueToken(t, defaultAudience, "b", time.Hour)
				_, err = val.ValidateToken(t.Context(), token)
				require.NoError(t, err)

				require.EqualValues(t, 2, idp.configRequests.Load())
				require.EqualValues(t, 1, idp.jwksRequests.Load())
			},
		},
		{
			name:     "stale config",
			audience: defaultAudience,
			execute: func(t *testing.T, idp *fakeIDP, v *CachingTokenValidator[*oidc.TokenClaims, StandardValidatorKey]) {
				val, err := v.GetValidatorWithKey(t.Context(), NewStandardValidatorKey(idp.issuer(), defaultAudience))
				require.NoError(t, err)
				val.verifierFn = minimalValidator()

				idp.clock.Advance(validatorTTL + time.Minute)

				token := idp.issueToken(t, defaultAudience, "a", time.Hour)
				_, err = val.ValidateToken(t.Context(), token)
				require.NoError(t, err)

				// This validation attempt should fetch both the config and JWKS
				// endpoint.
				require.EqualValues(t, 1, idp.configRequests.Load())
				require.EqualValues(t, 1, idp.jwksRequests.Load())

				idp.clock.Advance(discoveryTTL + time.Minute)
				token = idp.issueToken(t, defaultAudience, "b", time.Hour)
				_, err = val.ValidateToken(t.Context(), token)
				require.NoError(t, err)

				// Config should be reloaded, but the keyset will remain cached
				require.EqualValues(t, 2, idp.configRequests.Load())
				require.EqualValues(t, 1, idp.jwksRequests.Load())
			},
		},
		{
			name:     "changed jwks uri",
			audience: defaultAudience,
			execute: func(t *testing.T, idp *fakeIDP, v *CachingTokenValidator[*oidc.TokenClaims, StandardValidatorKey]) {
				val, err := v.GetValidatorWithKey(t.Context(), NewStandardValidatorKey(idp.issuer(), defaultAudience))
				require.NoError(t, err)
				val.verifierFn = minimalValidator()

				idp.clock.Advance(validatorTTL + time.Minute)

				token := idp.issueToken(t, defaultAudience, "a", time.Hour)
				_, err = val.ValidateToken(t.Context(), token)
				require.NoError(t, err)

				// This validation attempt should fetch both the config and JWKS
				// endpoint.
				require.EqualValues(t, 1, idp.configRequests.Load())
				require.EqualValues(t, 1, idp.jwksRequests.Load())

				// Switch to the new endpoint, advance the clock enough to
				// trigger a config refresh, and validate again.
				idp.useAlternateJWKSEndpoint.Store(true)
				idp.clock.Advance(discoveryTTL + time.Minute)
				token = idp.issueToken(t, defaultAudience, "b", time.Hour)
				_, err = val.ValidateToken(t.Context(), token)
				require.NoError(t, err)

				// Config should be reloaded, and the keyset should be reloaded.
				require.EqualValues(t, 2, idp.configRequests.Load())
				require.EqualValues(t, 2, idp.jwksRequests.Load())
			},
		},
		{
			name:     "validator pruning",
			audience: defaultAudience,
			execute: func(t *testing.T, idp *fakeIDP, v *CachingTokenValidator[*oidc.TokenClaims, StandardValidatorKey]) {
				valOld, err := v.GetValidatorWithKey(t.Context(), NewStandardValidatorKey(idp.issuer(), "a"))
				require.NoError(t, err)

				// After just 1 hour, it should return the same pointer
				idp.clock.Advance(time.Hour + time.Minute)

				valTemp, err := v.GetValidatorWithKey(t.Context(), NewStandardValidatorKey(idp.issuer(), "a"))
				require.NoError(t, err)
				require.Same(t, valOld, valTemp)

				// After 48 hours, make the request again. It's now past its
				// TTL and should be recreated.
				idp.clock.Advance(validatorTTL + time.Minute)
				valNew, err := v.GetValidatorWithKey(t.Context(), NewStandardValidatorKey(idp.issuer(), "a"))
				require.NoError(t, err)
				require.NotSame(t, valNew, valOld)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := clockwork.NewFakeClock()
			idp := newFakeIDP(t, clock, tt.audience)

			validator, err := NewCachingTokenValidator[*oidc.TokenClaims, StandardValidatorKey](clock)
			require.NoError(t, err)

			tt.execute(t, idp, validator)
		})
	}
}

func TestCachingTokenValidatorCustomKeys(t *testing.T) {
	type customKey struct {
		StandardValidatorKey

		custom string
	}

	newCustomKey := func(custom string) customKey {
		return customKey{
			StandardValidatorKey: StandardValidatorKey{
				issuer:   "issuer",
				audience: "audience",
			},
			custom: custom,
		}
	}

	ctx := t.Context()
	clock := clockwork.NewFakeClock()

	validator, err := NewCachingTokenValidator[*oidc.TokenClaims, customKey](clock)
	require.NoError(t, err)

	// Make sure inherited iss/aud are sane
	fooKey := newCustomKey("foo")
	require.Equal(t, "issuer", fooKey.GetIssuer())
	require.Equal(t, "audience", fooKey.GetAudience())

	foo, err := validator.GetValidatorWithKey(ctx, fooKey)
	require.NoError(t, err)

	foo2, err := validator.GetValidatorWithKey(ctx, fooKey)
	require.NoError(t, err)

	barKey := newCustomKey("bar")
	bar, err := validator.GetValidatorWithKey(ctx, barKey)
	require.NoError(t, err)

	require.Same(t, foo, foo2, "pointers with same cache key must be equal")
	require.NotSame(t, foo, bar, "pointers with different cache key must not be equal")
}

// countingRoundTripper is an http.RoundTripper that counts requests and
// delegates to a wrapped transport.
type countingRoundTripper struct {
	next  http.RoundTripper
	count atomic.Uint32
}

func (c *countingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	c.count.Add(1)
	return c.next.RoundTrip(req)
}

// TestCachingTokenValidatorWithClientMutator ensures HTTP client mutators are
// actually applied when new validators are created.
func TestCachingTokenValidatorWithClientMutator(t *testing.T) {
	t.Parallel()

	const defaultAudience = "example.teleport.sh"

	t.Run("mutator is applied to the live client", func(t *testing.T) {
		clock := clockwork.NewFakeClock()
		idp := newFakeIDP(t, clock, defaultAudience)

		validator, err := NewCachingTokenValidator[*oidc.TokenClaims, StandardValidatorKey](clock)
		require.NoError(t, err)

		// Mutate the client to include the countingRoundTripper.
		crt := &countingRoundTripper{}
		mutator := func(client *http.Client) error {
			crt.next = client.Transport
			client.Transport = crt
			return nil
		}

		val, err := validator.GetValidatorWithKey(
			t.Context(),
			NewStandardValidatorKey(idp.issuer(), defaultAudience),
			mutator,
		)
		require.NoError(t, err)

		// Should start empty...
		require.Zero(t, crt.count.Load())

		token := idp.issueToken(t, defaultAudience, "a", time.Hour)
		claims, err := val.ValidateToken(t.Context(), token)
		require.NoError(t, err)
		require.Equal(t, "a", claims.Subject)

		// Validation triggers a discovery fetch and a JWKS fetch, both of
		// which must go through our wrapped transport.
		require.EqualValues(t, idp.configRequests.Load()+idp.jwksRequests.Load(), crt.count.Load())
		require.Greater(t, crt.count.Load(), uint32(0))
	})

	t.Run("mutator is not applied on cache hit", func(t *testing.T) {
		clock := clockwork.NewFakeClock()
		idp := newFakeIDP(t, clock, defaultAudience)

		validator, err := NewCachingTokenValidator[*oidc.TokenClaims, StandardValidatorKey](clock)
		require.NoError(t, err)

		// Mutate the client to include the countingRoundTripper.
		crt := &countingRoundTripper{}
		mutator := func(client *http.Client) error {
			crt.next = client.Transport
			client.Transport = crt
			return nil
		}

		// Get without a mutator first.
		val, err := validator.GetValidatorWithKey(
			t.Context(),
			NewStandardValidatorKey(idp.issuer(), defaultAudience),
		)
		require.NoError(t, err)

		// As before, count should start empty.
		require.Zero(t, crt.count.Load())

		token := idp.issueToken(t, defaultAudience, "a", time.Hour)
		claims, err := val.ValidateToken(t.Context(), token)
		require.NoError(t, err)
		require.Equal(t, "a", claims.Subject)

		// Should not be incremented.
		require.Zero(t, crt.count.Load(), uint32(0))

		// Fetch again but include the mutator
		val, err = validator.GetValidatorWithKey(
			t.Context(),
			NewStandardValidatorKey(idp.issuer(), defaultAudience),
			mutator,
		)
		require.NoError(t, err)

		// Issue and validate again...
		token = idp.issueToken(t, defaultAudience, "a", time.Hour)
		claims, err = val.ValidateToken(t.Context(), token)
		require.NoError(t, err)
		require.Equal(t, "a", claims.Subject)

		// Still should not be incremented, despite including the mutator.
		require.Zero(t, crt.count.Load())
	})

	t.Run("mutator error fails construction", func(t *testing.T) {
		clock := clockwork.NewFakeClock()
		idp := newFakeIDP(t, clock, defaultAudience)

		validator, err := NewCachingTokenValidator[*oidc.TokenClaims, StandardValidatorKey](clock)
		require.NoError(t, err)

		mutator := func(client *http.Client) error {
			return trace.BadParameter("fail")
		}

		_, err = validator.GetValidatorWithKey(
			t.Context(),
			NewStandardValidatorKey(idp.issuer(), defaultAudience),
			mutator,
		)
		require.ErrorContains(t, err, "fail")
	})
}
