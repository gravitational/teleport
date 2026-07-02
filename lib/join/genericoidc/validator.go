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

package genericoidc

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/go-jose/go-jose/v4"
	josejwt "github.com/go-jose/go-jose/v4/jwt"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/join/provision"
	"github.com/gravitational/teleport/lib/oidc"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, teleport.Component("genericoidc"))

// acceptableStaticJWKSAlgs is a list of algorithms considered acceptable for
// static_jwks verification attempts. This only includes asymmetric algorithms,
// see also:
// - https://trufflesecurity.com/blog/stop-recommending-jwts
// - https://datatracker.ietf.org/doc/html/rfc8725#section-2.1
// This list isn't passed directly to jose, instead we compute the intersection
// of this set and the set of algorithms specified in the static_jwks keys.
// Additionally, note that all of these algorithms are technically allowable for
// verification in FIPS builds, however some may be rejected by BoringCrypto
// (especially EdDSA) or may qualify as allowed but legacy verification
// (RS* /PS* < 2048).
var acceptableStaticJWKSAlgs = map[jose.SignatureAlgorithm]struct{}{
	jose.RS256: {},
	jose.RS384: {},
	jose.RS512: {},

	jose.PS256: {},
	jose.PS384: {},
	jose.PS512: {},

	jose.ES256: {},
	jose.ES384: {},
	jose.ES512: {},

	jose.EdDSA: {},
}

const (
	// httpTimeout is a limit on the amount of time any single HTTP request can
	// take when discovering OIDC parameters (configuration, JWKS, etc).
	httpTimeout = 10 * time.Second
)

// GenericOIDCKey is the client cache key used to identify a particular cached
// OIDC client instance. This additionally includes a hash of the TLS CA (if
// any) to ensure a new client is constructed if the CA is changed.
type GenericOIDCKey struct {
	oidc.StandardValidatorKey

	// tlsCAHash is a hash of the TLS CA config (which may be empty), used to
	// invalidate validator instances if the user updates their CA.
	tlsCAHash string
}

// IDTokenValidator can be used to validate generic OIDC tokens.
type IDTokenValidator struct {
	validator *oidc.CachingTokenValidator[*IDTokenClaims, GenericOIDCKey]
}

// ValidateToken validates a generic OIDC token using a remote, cached OIDC
// endpoint
func (v *IDTokenValidator) ValidateToken(
	ctx context.Context,
	provisionToken provision.Token,
	idToken []byte,
) (*IDTokenClaims, error) {
	spec, err := provisionToken.GetGenericOIDC()
	if err != nil {
		return nil, trace.Wrap(err, "retrieving normalized generic_oidc token configuration")
	}

	var claims *IDTokenClaims
	if spec.StaticJWKS != "" {
		claims, err = v.validateStaticJWKS(ctx, spec, idToken, time.Now())
		if err != nil {
			return nil, trace.Wrap(err, "validating via static_jwks")
		}
	} else {
		claims, err = v.validateOIDC(ctx, spec, idToken)
		if err != nil {
			log.InfoContext(ctx, "denying generic_oidc join attempt", "error", err)
			return nil, trace.Wrap(err, "validating via oidc")
		}
	}

	if err := evaluateGenericOIDCRules(spec, claims); err != nil {
		return nil, trace.Wrap(err, "validating generic_oidc rules")
	}

	return claims, nil
}

// keyForToken returns a caching for the given token, used to fetch the existing
// cached OIDC validator, if any, or to identify a new one if necessary.
func keyForToken(spec *types.ProvisionTokenSpecV2GenericOIDC) GenericOIDCKey {
	key := GenericOIDCKey{
		StandardValidatorKey: oidc.NewStandardValidatorKey(spec.Issuer, spec.Audience),
	}

	if spec.TLSCA != "" {
		hash := sha256.Sum256([]byte(spec.TLSCA))
		key.tlsCAHash = hex.EncodeToString(hash[:])
	}

	return key
}

func (v *IDTokenValidator) validateOIDC(
	ctx context.Context,
	spec *types.ProvisionTokenSpecV2GenericOIDC,
	idToken []byte,
) (*IDTokenClaims, error) {
	key := keyForToken(spec)

	validator, err := v.validator.GetValidatorWithKey(ctx, key, func(client *http.Client) error {
		transport, err := defaults.Transport()
		if err != nil {
			return trace.Wrap(err)
		}

		if len(spec.TLSCA) > 0 {
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM([]byte(spec.TLSCA)) {
				return trace.BadParameter("no valid certificates in `tls_ca`")
			}

			transport.TLSClientConfig = &tls.Config{
				RootCAs: pool,
			}
		}

		client.Timeout = httpTimeout
		client.Transport = oidc.NewOIDCRoundTripper(otelhttp.NewTransport(transport))
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We always skip AZP verification. Per the spec, this check is optional,
	// and the actual check as implemented by zitadel/oidc is a simple string
	// compare; if users want to check azp, they can add a check rule
	// themselves.
	withoutAZPVerifier := rp.WithAZPVerifier(func(string) error {
		return nil
	})

	claims, err := validator.ValidateToken(ctx, string(idToken), withoutAZPVerifier)
	if err != nil {
		return nil, trace.Wrap(err, "validating OIDC token")
	}

	return claims, nil
}

func (v *IDTokenValidator) validateStaticJWKS(
	ctx context.Context,
	spec *types.ProvisionTokenSpecV2GenericOIDC,
	idToken []byte,
	now time.Time,
) (*IDTokenClaims, error) {
	jwks := jose.JSONWebKeySet{}
	if err := json.Unmarshal([]byte(spec.StaticJWKS), &jwks); err != nil {
		return nil, trace.Wrap(err, "parsing provided jwks")
	}

	algs, filteredKeys := filterAllowedAlgorithmsFromJWKS(jwks)
	if len(algs) == 0 {
		return nil, trace.BadParameter("static_jwks contains no keys with a usable signature algorithm")
	}

	parsed, err := josejwt.ParseSigned(string(idToken), algs)
	if err != nil {
		return nil, trace.Wrap(err, "parsing jwt")
	}

	// Parse claims both to stdClaims (for validation) and our "custom" claims.
	// Note that go-jose/v4 requires that `kid` is set. It's technically
	// possible for an issuer to omit `kid` and serve just one (technically
	// unambiguous) key, which will then fail validation. We'll fail on that for
	// now, and can reconsider if we encounter an IdP in the wild that does
	// this.
	stdClaims := josejwt.Claims{}
	claims := IDTokenClaims{}
	if err := parsed.Claims(filteredKeys, &stdClaims, &claims); err != nil {
		if errors.Is(err, jose.ErrJWKSKidNotFound) {
			log.WarnContext(
				ctx, "unable to validate incoming jwt without a `kid`",
				"error", err,
			)
		}
		return nil, trace.Wrap(err, "validating jwt signature")
	}

	// go-jose/v4 only checks exp/iat/sub/nbf when present, so make sure they
	// are present manually so we can be sure they'll be validated, for parity
	// with the OIDC path. We'll leave `nbf` optional as it is also optional for
	// OIDC.
	if stdClaims.Expiry == nil {
		return nil, trace.AccessDenied("token must have an `exp` claim")
	}

	if stdClaims.IssuedAt == nil {
		return nil, trace.AccessDenied("token must have an `iat` claim")
	}

	if stdClaims.Subject == "" {
		return nil, trace.AccessDenied("token must have a `sub` claim")
	}

	leeway := time.Second * 10
	err = stdClaims.ValidateWithLeeway(josejwt.Expected{
		Issuer:      spec.Issuer,
		AnyAudience: josejwt.Audience{spec.Audience},
		Time:        now,
	}, leeway)
	if err != nil {
		return nil, trace.Wrap(err, "validating standard claims")
	}

	return &claims, nil
}

// allowedAlgorithmsFromJWKS computes the list of algorithms we should pass to
// jose, by computing the intersection of our allowed algorithms
// (acceptableStaticJWKSAlgs) and the algorithms in the `static_jwks` keys.
func filterAllowedAlgorithmsFromJWKS(jwks jose.JSONWebKeySet) ([]jose.SignatureAlgorithm, jose.JSONWebKeySet) {
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
			// exploit path especially given the nature of static_jwks keys.
			continue
		}

		acceptable := false
		alg := jose.SignatureAlgorithm(key.Algorithm)
		if _, ok := acceptableStaticJWKSAlgs[alg]; ok {
			candidates[alg] = struct{}{}
			acceptable = true
		} else if key.Algorithm == "" {
			// `alg` is optional, so if not specified, include based on the key
			// type
			// https://datatracker.ietf.org/doc/html/rfc7517#section-4.4
			switch k := key.Key.(type) {
			case *rsa.PublicKey:
				// Note, technically ambiguous between 256/384/512, but without
				// a hint we have to guess, and RS256 is mandated.
				candidates[jose.RS256] = struct{}{}
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

// evaluateGenericOIDCRules evaluates all types of user-defined rules on a
// generic_oidc-type token, following their defined evaluation order and
// semantics. It also performs final rule-sanity checks to ensure the end user
// has configured at least some proper rule.
func evaluateGenericOIDCRules(spec *types.ProvisionTokenSpecV2GenericOIDC, claims *IDTokenClaims) error {
	allowAnyHasAny := len(spec.AllowAny) > 0

	var err error
	mustMatchFieldsHasAny := false
	if spec.MustMatchFields != nil {
		// We can't trust non-nil to actually have a useful rule. Instead, we'll
		// use an explicit helper that traverses the struct to make sure an
		// actual comparison is made somewhere in the tree.
		mustMatchFieldsHasAny, err = validateFieldRulesContainsAnyRule(spec.MustMatchFields)
		if err != nil {
			return trace.Wrap(err, "validating must_match_fields")
		}
	}

	// If no rules were defined, this token is invalid and cannot be used.
	if !mustMatchFieldsHasAny && !allowAnyHasAny {
		return trace.BadParameter("generic OIDC token has no rules configured")
	}

	// Always try to evaluate field rules, regardless of `mustMatchFieldsHasAny`
	if err := evaluateFieldRules(spec.MustMatchFields, claims.Claims); err != nil {
		return trace.Wrap(err, "validating field rules")
	}

	// If allow rules were defined, evaluate them last.
	if allowAnyHasAny {
		if err := evaluateAllowAnyRules(spec, claims); err != nil {
			return trace.Wrap(err, "validating allow rules")
		}
	}

	return nil
}

// evaluateAllowAnyRules evaluates rules under `allow_any`, returning
// successfully for the first entry that does not return an error or an explicit
// denial
func evaluateAllowAnyRules(spec *types.ProvisionTokenSpecV2GenericOIDC, claims *IDTokenClaims) error {
	for i, rule := range spec.AllowAny {
		switch {
		case len(rule.Conditions) > 0 && rule.Expression != "":
			return trace.BadParameter("allow_any[%d]: cannot contain both `conditions` and `expression`", i)
		case len(rule.Conditions) > 0:
			err := evaluateAllowAnyConditions(rule.Conditions, claims)
			if err != nil && !trace.IsAccessDenied(err) {
				// non access denied error (e.g. structurally invalid rule),
				// return early
				return trace.Wrap(err, "allow_any[%d]: evaluating conditions", i)
			}

			if err == nil {
				// all conditions passed, allow the attempt
				return nil
			}
		case rule.Expression != "":
			result, err := evaluateExpression(rule.Expression, &Environment{
				Claims: claims.Claims,
			})
			if err != nil {
				// Expressions return an explicit allowed bool so we return on
				// any error. Technically we could continue trying other rules,
				// but for now we'll raise excessive errors to convince the user
				// to fix their broken expression.
				return trace.Wrap(err, "allow_any[%d]: evaluating allow expression", i)
			}

			if result {
				return nil
			}
		default:
			return trace.BadParameter("allow_any[%d]: exactly one of `conditions` or `expression` must be set", i)
		}
	}

	return trace.AccessDenied("claims matched no allow_any rules")
}

// newIDTokenValidatorWithClock constructs an IDTokenValidator.
func newIDTokenValidatorWithClock(clock clockwork.Clock) (*IDTokenValidator, error) {
	validator, err := oidc.NewCachingTokenValidator[*IDTokenClaims, GenericOIDCKey](clock)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &IDTokenValidator{
		validator: validator,
	}, nil
}

// NewOIDCTokenValidator constructs an IDTokenValidator.
func NewIDTokenValidator() (*IDTokenValidator, error) {
	return newIDTokenValidatorWithClock(clockwork.NewRealClock())
}
