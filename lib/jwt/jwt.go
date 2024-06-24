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

// Package jwt is used to sign and verify JWT tokens used by application access.
package jwt

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/coreos/go-oidc"
	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/cryptosigner"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/keys"
)

// Config defines the clock and PEM encoded bytes of a public and private
// key that form a *jwt.Key.
type Config struct {
	// Clock is used to control expiry time.
	Clock clockwork.Clock

	// PublicKey is used to verify a signed token.
	PublicKey crypto.PublicKey

	// PrivateKey is used to sign and verify tokens.
	PrivateKey crypto.Signer

	// Algorithm is algorithm used to sign JWT tokens.
	Algorithm jose.SignatureAlgorithm

	// ClusterName is the name of the cluster that will be signing the JWT tokens.
	ClusterName string
}

// CheckAndSetDefaults validates the values of a *Config.
func (c *Config) CheckAndSetDefaults() error {
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.PrivateKey != nil {
		c.PublicKey = c.PrivateKey.Public()
	}

	if c.PrivateKey == nil && c.PublicKey == nil {
		return trace.BadParameter("public or private key is required")
	}
	if c.Algorithm == "" {
		return trace.BadParameter("algorithm is required")
	}
	if c.ClusterName == "" {
		return trace.BadParameter("cluster name is required")
	}

	return nil
}

// Key is a JWT key that can be used to sign and/or verify a token.
type Key struct {
	config *Config
}

// New creates a JWT key that can be used to sign and verify tokens.
func New(config *Config) (*Key, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Key{
		config: config,
	}, nil
}

// SignParams are the claims to be embedded within the JWT token.
type SignParams struct {
	// Username is the Teleport identity.
	Username string

	// Roles are the roles assigned to the user within Teleport.
	Roles []string

	// Traits are the traits assigned to the user within Teleport.
	Traits wrappers.Traits

	// Expiry is time to live for the token.
	Expires time.Time

	// URI is the URI of the recipient application.
	URI string

	// Audience is the Audience for the Token.
	Audience string

	// Issuer is the issuer of the token.
	Issuer string

	// Subject is the system that is going to use the token.
	Subject string
}

// Check verifies all the values are valid.
func (p *SignParams) Check() error {
	if p.Username == "" {
		return trace.BadParameter("username missing")
	}
	if p.Expires.IsZero() {
		return trace.BadParameter("expires missing")
	}
	if p.URI == "" {
		return trace.BadParameter("uri missing")
	}

	return nil
}

// sign will return a signed JWT with the passed in claims embedded within.
// `opts`, when not nil, specifies additional signing options, such as additional JWT headers.
func (k *Key) sign(claims any, opts *jose.SignerOptions) (string, error) {
	return k.signAny(claims, opts)
}

// signAny will return a signed JWT with the passed in claims embedded within; unlike sign it allows more flexibility in the claim data.
func (k *Key) signAny(claims any, opts *jose.SignerOptions) (string, error) {
	if k.config.PrivateKey == nil {
		return "", trace.BadParameter("can not sign token with non-signing key")
	}

	// Create a signer with configured private key and algorithm.
	var signer interface{}
	switch k.config.PrivateKey.(type) {
	case *rsa.PrivateKey:
		signer = k.config.PrivateKey
	default:
		signer = cryptosigner.Opaque(k.config.PrivateKey)
	}
	signingKey := jose.SigningKey{
		Algorithm: k.config.Algorithm,
		Key:       signer,
	}

	if opts == nil {
		opts = &jose.SignerOptions{}
	}
	opts = opts.WithType("JWT")
	sig, err := jose.NewSigner(signingKey, opts)
	if err != nil {
		return "", trace.Wrap(err)
	}

	token, err := jwt.Signed(sig).Claims(claims).CompactSerialize()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return token, nil
}

func (k *Key) Sign(p SignParams) (string, error) {
	if err := p.Check(); err != nil {
		return "", trace.Wrap(err)
	}

	// Sign the claims and create a JWT token.
	claims := Claims{
		Claims: jwt.Claims{
			Subject:   p.Username,
			Issuer:    k.config.ClusterName,
			Audience:  jwt.Audience{p.URI},
			NotBefore: jwt.NewNumericDate(k.config.Clock.Now().Add(-10 * time.Second)),
			IssuedAt:  jwt.NewNumericDate(k.config.Clock.Now()),
			Expiry:    jwt.NewNumericDate(p.Expires),
		},
		Username: p.Username,
		Roles:    p.Roles,
		Traits:   p.Traits,
	}

	return k.sign(claims, nil)
}

// awsOIDCCustomClaims defines the require claims for the JWT token used in AWS OIDC Integration.
type awsOIDCCustomClaims struct {
	jwt.Claims

	// OnBehalfOf identifies the user that is started the request.
	OnBehalfOf string `json:"obo,omitempty"`
}

// SignAWSOIDC signs a JWT with claims specific to AWS OIDC Integration.
// Required Params:
// - Username: stored as OnBehalfOf (obo) claim with `user:` prefix
// - Issuer: stored as Issuer (iss) claim
// - Subject: stored as Subject (sub) claim
// - Audience: stored as Audience (aud) claim
// - Expires: stored as Expiry (exp) claim
func (k *Key) SignAWSOIDC(p SignParams) (string, error) {
	// Sign the claims and create a JWT token.
	claims := awsOIDCCustomClaims{
		OnBehalfOf: "user:" + p.Username,
		Claims: jwt.Claims{
			Issuer:    p.Issuer,
			Subject:   p.Subject,
			Audience:  jwt.Audience{p.Audience},
			ID:        uuid.NewString(),
			NotBefore: jwt.NewNumericDate(k.config.Clock.Now().Add(-10 * time.Second)),
			Expiry:    jwt.NewNumericDate(p.Expires),
			IssuedAt:  jwt.NewNumericDate(k.config.Clock.Now().Add(-10 * time.Second)),
		},
	}

	// AWS does not require `kid` claim in the JWT per se,
	// but it seems to (NB: educated guess) require it if JWKS has multiple JWK-s with different `kid`-s.
	opts := (&jose.SignerOptions{}).
		WithHeader(jose.HeaderKey("kid"), "")

	return k.sign(claims, opts)
}

// SignEntraOIDC signs a JWT for the Entra ID Integration.
// Required Params:
// - Issuer: stored as Issuer (iss) claim
// - Subject: stored as Subject (sub) claim
// - Audience: stored as Audience (aud) claim
// - Expires: stored as Expiry (exp) claim
func (k *Key) SignEntraOIDC(p SignParams) (string, error) {
	// Sign the claims and create a JWT token.
	claims := jwt.Claims{
		Issuer:    p.Issuer,
		Subject:   p.Subject,
		Audience:  jwt.Audience{p.Audience},
		ID:        uuid.NewString(),
		NotBefore: jwt.NewNumericDate(k.config.Clock.Now().Add(-10 * time.Second)),
		Expiry:    jwt.NewNumericDate(p.Expires),
		IssuedAt:  jwt.NewNumericDate(k.config.Clock.Now().Add(-10 * time.Second)),
	}

	// Azure expect a `kid` header to be present and non-empty,
	// unlike e.g. AWS which accepts an empty `kid` string value.
	publicKey, ok := k.config.PublicKey.(*rsa.PublicKey)
	if !ok {
		return "", trace.BadParameter("expected an RSA public key")
	}
	kid := KeyID(publicKey)
	opts := (&jose.SignerOptions{}).
		WithHeader(jose.HeaderKey("kid"), kid)
	return k.sign(claims, opts)
}

func (k *Key) SignSnowflake(p SignParams, issuer string) (string, error) {
	// Sign the claims and create a JWT token.
	claims := Claims{
		Claims: jwt.Claims{
			Subject:   p.Username,
			Issuer:    issuer,
			NotBefore: jwt.NewNumericDate(k.config.Clock.Now().Add(-10 * time.Second)),
			Expiry:    jwt.NewNumericDate(p.Expires),
			IssuedAt:  jwt.NewNumericDate(k.config.Clock.Now().Add(-10 * time.Second)),
		},
	}

	return k.sign(claims, nil)
}

// AzureTokenClaims represent a minimal set of claims that will be encoded as JWT in Azure access token and passed back to az CLI.
type AzureTokenClaims struct {
	// TenantID represents TenantID; this is read by az CLI.
	TenantID string `json:"tid"`
	// Resource records the resource requested by az CLI. This will be used in backend to request real token with appropriate scope.
	Resource string `json:"resource"`
}

// SignAzureToken signs AzureTokenClaims
func (k *Key) SignAzureToken(claims AzureTokenClaims) (string, error) {
	return k.signAny(claims, nil)
}

type PROXYSignParams struct {
	ClusterName        string
	SourceAddress      string
	DestinationAddress string
}

const expirationPROXY = time.Second * 60

// SignPROXYJwt will create short lived signed JWT that is used in signed PROXY header
func (k *Key) SignPROXYJWT(p PROXYSignParams) (string, error) {
	claims := Claims{
		Claims: jwt.Claims{
			Subject:   p.SourceAddress,
			Audience:  []string{p.DestinationAddress},
			Issuer:    p.ClusterName,
			NotBefore: jwt.NewNumericDate(k.config.Clock.Now().Add(-10 * time.Second)),
			Expiry:    jwt.NewNumericDate(k.config.Clock.Now().Add(expirationPROXY)),
			IssuedAt:  jwt.NewNumericDate(k.config.Clock.Now()),
		},
	}

	return k.sign(claims, nil)
}

// VerifyParams are the parameters needed to pass the token and data needed to verify.
type VerifyParams struct {
	// Username is the Teleport identity.
	Username string

	// RawToken is the JWT token.
	RawToken string

	// URI is the URI of the recipient application.
	URI string

	// Audience is the Audience for the token
	Audience string
}

// Check verifies all the values are valid.
func (p *VerifyParams) Check() error {
	if p.Username == "" {
		return trace.BadParameter("username missing")
	}
	if p.RawToken == "" {
		return trace.BadParameter("raw token missing")
	}
	if p.URI == "" {
		return trace.BadParameter("uri missing")
	}

	return nil
}

type SnowflakeVerifyParams struct {
	AccountName string
	LoginName   string
	RawToken    string
}

func (p *SnowflakeVerifyParams) Check() error {
	if p.AccountName == "" {
		return trace.BadParameter("account name missing")
	}

	if p.LoginName == "" {
		return trace.BadParameter("login name is missing")
	}

	if p.RawToken == "" {
		return trace.BadParameter("raw token missing")
	}

	return nil
}

type PROXYVerifyParams struct {
	ClusterName        string
	SourceAddress      string
	DestinationAddress string
	RawToken           string
}

func (p *PROXYVerifyParams) Check() error {
	if p.ClusterName == "" {
		return trace.BadParameter("cluster name missing")
	}
	if p.SourceAddress == "" {
		return trace.BadParameter("source address missing")
	}

	return nil
}

func (k *Key) verify(rawToken string, expectedClaims jwt.Expected) (*Claims, error) {
	if k.config.PublicKey == nil {
		return nil, trace.BadParameter("can not verify token without public key")
	}
	// Parse the token.
	tok, err := jwt.ParseSigned(rawToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Validate the signature on the JWT token.
	var out Claims
	if err := tok.Claims(k.config.PublicKey, &out); err != nil {
		return nil, trace.Wrap(err)
	}

	// Validate the claims on the JWT token.
	if err = out.Validate(expectedClaims); err != nil {
		return nil, trace.Wrap(err)
	}

	return &out, nil
}

// Verify will validate the passed in JWT token.
func (k *Key) Verify(p VerifyParams) (*Claims, error) {
	if err := p.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	expectedClaims := jwt.Expected{
		Issuer:   k.config.ClusterName,
		Subject:  p.Username,
		Audience: jwt.Audience{p.URI},
		Time:     k.config.Clock.Now(),
	}

	return k.verify(p.RawToken, expectedClaims)
}

// AWSOIDCVerifyParams are the params required to verify an AWS OIDC Token.
type AWSOIDCVerifyParams struct {
	RawToken string
	Issuer   string
}

// Check ensures all the required fields are present.
func (p *AWSOIDCVerifyParams) Check() error {
	if p.RawToken == "" {
		return trace.BadParameter("raw token is missing")
	}

	if p.Issuer == "" {
		return trace.BadParameter("issuer is missing")
	}

	return nil
}

// VerifyAWSOIDC will validate the passed in JWT token for the AWS OIDC Integration
func (k *Key) VerifyAWSOIDC(p AWSOIDCVerifyParams) (*Claims, error) {
	if err := p.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	expectedClaims := jwt.Expected{
		Issuer:   p.Issuer,
		Subject:  types.IntegrationAWSOIDCSubject,
		Audience: jwt.Audience{types.IntegrationAWSOIDCAudience},
		Time:     k.config.Clock.Now(),
	}

	return k.verify(p.RawToken, expectedClaims)
}

// VerifyPROXY will validate the passed JWT for signed PROXY header
func (k *Key) VerifyPROXY(p PROXYVerifyParams) (*Claims, error) {
	if err := p.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	expectedClaims := jwt.Expected{
		Issuer:   p.ClusterName,
		Subject:  p.SourceAddress,
		Audience: []string{p.DestinationAddress},
		Time:     k.config.Clock.Now(),
	}

	return k.verify(p.RawToken, expectedClaims)
}

// VerifySnowflake will validate the passed in JWT token.
func (k *Key) VerifySnowflake(p SnowflakeVerifyParams) (*Claims, error) {
	if err := p.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	pubKey, err := x509.MarshalPKIXPublicKey(k.config.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	keyFp := sha256.Sum256(pubKey)
	keyFpStr := base64.StdEncoding.EncodeToString(keyFp[:])

	accName := strings.ToUpper(p.AccountName)
	loginName := strings.ToUpper(p.LoginName)

	// Generate issuer name in the Snowflake required format.
	issuer := fmt.Sprintf("%s.%s.SHA256:%s", accName, loginName, keyFpStr)

	// Validate the claims on the JWT token.
	expectedClaims := jwt.Expected{
		Issuer:  issuer,
		Subject: fmt.Sprintf("%s.%s", accName, loginName),
		Time:    k.config.Clock.Now(),
	}
	return k.verify(p.RawToken, expectedClaims)
}

func (k *Key) VerifyAzureToken(rawToken string) (*AzureTokenClaims, error) {
	if k.config.PublicKey == nil {
		return nil, trace.BadParameter("can not verify token without public key")
	}
	// Parse the token.
	tok, err := jwt.ParseSigned(rawToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Validate the signature on the JWT token.
	var out AzureTokenClaims
	if err := tok.Claims(k.config.PublicKey, &out); err != nil {
		return nil, trace.Wrap(err)
	}

	return &out, nil
}

// Claims represents public and private claims for a JWT token.
type Claims struct {
	// Claims represents public claim values (as specified in RFC 7519).
	jwt.Claims

	// Username returns the Teleport identity of the user.
	Username string `json:"username"`

	// Roles returns the list of roles assigned to the user within Teleport.
	Roles []string `json:"roles"`

	// Traits returns the traits assigned to the user within Teleport.
	Traits wrappers.Traits `json:"traits"`
}

// GenerateKeyPair generates and return a PEM encoded private and public
// key in the format used by this package.
func GenerateKeyPair() ([]byte, []byte, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, constants.RSAKeySize)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	public, err := keys.MarshalPublicKey(privateKey.Public())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	private, err := keys.MarshalPrivateKey(privateKey)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return public, private, nil
}

// CheckNotBefore ensures the token was not issued in the future.
// https://www.rfc-editor.org/rfc/rfc7519#section-4.1.5
// 4.1.5.  "nbf" (Not Before) Claim
// TODO(strideynet): upstream support for `nbf` into the go-oidc lib.
func CheckNotBefore(now time.Time, leeway time.Duration, token *oidc.IDToken) error {
	claims := struct {
		NotBefore *JSONTime `json:"nbf"`
	}{}
	if err := token.Claims(&claims); err != nil {
		return trace.Wrap(err)
	}

	if claims.NotBefore != nil {
		adjustedNow := now.Add(leeway)
		nbf := time.Time(*claims.NotBefore)
		if adjustedNow.Before(nbf) {
			return trace.AccessDenied("token not before in future")
		}
	}

	return nil
}

// JSONTime unmarshaling sourced from https://github.com/gravitational/go-oidc/blob/master/oidc.go#L295
// TODO(strideynet): upstream support for `nbf` into the go-oidc lib.
type JSONTime time.Time

func (j *JSONTime) UnmarshalJSON(b []byte) error {
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	var unix int64

	if t, err := n.Int64(); err == nil {
		unix = t
	} else {
		f, err := n.Float64()
		if err != nil {
			return err
		}
		unix = int64(f)
	}
	*j = JSONTime(time.Unix(unix, 0))
	return nil
}
