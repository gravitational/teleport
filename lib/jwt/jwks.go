// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package jwt

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"maps"
	"slices"

	"github.com/go-jose/go-jose/v4"
)

// AllowedJWKSAlgorithms is a list of algorithms considered acceptable when
// verifying a token against a JWKS supplied by an external party, whether
// given out of band (e.g. a join method's static_jwks configuration) or
// fetched live from a trusted OIDC discovery endpoint. This only includes
// asymmetric algorithms, see also:
// - https://trufflesecurity.com/blog/stop-recommending-jwts
// - https://datatracker.ietf.org/doc/html/rfc8725#section-2.1
// Prefer FilterAllowedAlgorithmsFromJWKS when a JWKS is available: it
// computes the intersection of this list and the algorithms actually present
// in the JWKS. Use this list directly only when there is no JWKS to narrow
// against (e.g. a first, unverified pass over a token's claims via
// (*jwt.JSONWebToken).UnsafeClaimsWithoutVerification).
// Additionally, note that all of these algorithms are technically allowable
// for verification in FIPS builds, however some may be rejected by
// BoringCrypto (especially EdDSA) or may qualify as allowed but legacy
// verification (RS*/PS* < 2048).
var AllowedJWKSAlgorithms = []jose.SignatureAlgorithm{
	jose.RS256, jose.RS384, jose.RS512,
	jose.PS256, jose.PS384, jose.PS512,
	jose.ES256, jose.ES384, jose.ES512,
	jose.EdDSA,
}

// AllowedJWKSAlgorithmSet is AllowedJWKSAlgorithms as a set, for fast
// membership checks.
var AllowedJWKSAlgorithmSet = func() map[jose.SignatureAlgorithm]struct{} {
	m := make(map[jose.SignatureAlgorithm]struct{}, len(AllowedJWKSAlgorithms))
	for _, alg := range AllowedJWKSAlgorithms {
		m[alg] = struct{}{}
	}
	return m
}()

// FilterAllowedAlgorithmsFromJWKS computes the list of algorithms we should
// pass to jose, by computing the intersection of our allowed algorithms
// (AllowedJWKSAlgorithmSet) and the algorithms in the given JWKS's keys. It
// also returns a filtered JWKS containing only the keys that are usable for
// signature verification, i.e. keys whose `use` is unset or "sig", and whose
// algorithm (explicit or inferred from key type) is in the allowed set.
func FilterAllowedAlgorithmsFromJWKS(jwks jose.JSONWebKeySet) ([]jose.SignatureAlgorithm, jose.JSONWebKeySet) {
	filteredSet := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{},
	}

	candidates := make(map[jose.SignatureAlgorithm]struct{})
	for _, key := range jwks.Keys {
		if key.Use != "" && key.Use != "sig" {
			// If "use" is set and it isn't for signing, don't try to include
			// it. "sig" is a well known value per
			// https://datatracker.ietf.org/doc/html/rfc7517#section-4.2
			// Note that we honor `use` here but don't check `key_ops`. This
			// mirrors zitadel/oidc, and in practice since go-jose/v4 drops the
			// field, it isn't worth re-parsing for what is not a realistic
			// exploit path especially given the nature of static JWKS keys.
			continue
		}

		acceptable := false
		alg := jose.SignatureAlgorithm(key.Algorithm)
		if _, ok := AllowedJWKSAlgorithmSet[alg]; ok {
			candidates[alg] = struct{}{}
			acceptable = true
		} else if key.Algorithm == "" {
			// `alg` is optional, so if not specified, include based on the key
			// type
			// https://datatracker.ietf.org/doc/html/rfc7517#section-4.4
			switch k := key.Key.(type) {
			case *rsa.PublicKey:
				// An RSA key is usable with any of the RSA signature
				// algorithms, and `alg` is what would narrow that down. With
				// no `alg` to go on, infer all of them rather than assuming
				// RS256: a token legitimately signed with e.g. RS512 by this
				// key would otherwise be rejected by jwt.ParseSigned before
				// its signature is ever checked.
				for _, rsaAlg := range []jose.SignatureAlgorithm{
					jose.RS256, jose.RS384, jose.RS512,
					jose.PS256, jose.PS384, jose.PS512,
				} {
					candidates[rsaAlg] = struct{}{}
				}
				acceptable = true
			case *ecdsa.PublicKey:
				switch k.Curve {
				case elliptic.P256():
					candidates[jose.ES256] = struct{}{}
					acceptable = true
				case elliptic.P384():
					candidates[jose.ES384] = struct{}{}
					acceptable = true
				case elliptic.P521():
					// P521 -> ES512 (confusingly)
					// https://datatracker.ietf.org/doc/html/rfc7518#section-3.4
					candidates[jose.ES512] = struct{}{}
					acceptable = true
				}
			case ed25519.PublicKey:
				candidates[jose.EdDSA] = struct{}{}
				acceptable = true
			}

			// If nothing matches, it's a no-op, it certainly wouldn't appear in
			// our list.
		}

		if acceptable {
			filteredSet.Keys = append(filteredSet.Keys, key)
		}
	}

	return slices.Collect(maps.Keys(candidates)), filteredSet
}
