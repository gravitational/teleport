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

package jwt

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"math/big"

	"github.com/go-jose/go-jose/v3"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/defaults"
)

const (
	keyTypeRSA = "RSA"
	keyTypeEC  = "EC"
)

// JWK is a JSON Web Key, described in detail in RFC 7517.
type JWK struct {
	// KeyType is the type of asymmetric key used.
	KeyType string `json:"kty"`
	// Algorithm used to sign.
	Algorithm string `json:"alg"`

	// N is the modulus of an RSA public key.
	N string `json:"n,omitempty"`
	// E is the exponent of an RSA public key.
	E string `json:"e,omitempty"`

	// Curve identifies the cryptographic curve used with an ECDSA public key.
	Curve string `json:"crv,omitempty"`
	// X is the x coordinate parameter of an ECDSA public key.
	X string `json:"x,omitempty"`
	// Y is the y coordinate parameter of an ECDSA public key.
	Y string `json:"y,omitempty"`

	// Use identifies the intended use of the public key.
	// This field is required for the AWS OIDC Integration.
	// https://www.rfc-editor.org/rfc/rfc7517#section-4.2
	Use string `json:"use"`
	// KeyID identifies the key to use.
	// This field is required (even if empty) for the AWS OIDC Integration.
	// https://www.rfc-editor.org/rfc/rfc7517#section-4.5
	KeyID string `json:"kid"`
}

// KeyID returns a key id derived from the public key.
func KeyID(pub crypto.PublicKey) (string, error) {
	switch p := pub.(type) {
	case *rsa.PublicKey:
		return rsaKeyID(p), nil
	default:
		return genericKeyID(p)
	}
}

func rsaKeyID(pub *rsa.PublicKey) string {
	hash := sha256.Sum256(x509.MarshalPKCS1PublicKey(pub))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func genericKeyID(pub crypto.PublicKey) (string, error) {
	pubKeyDER, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", trace.Wrap(err)
	}
	hash := sha256.Sum256(pubKeyDER)
	return base64.RawURLEncoding.EncodeToString(hash[:]), nil
}

// MarshalJWK will marshal a supported public key into JWK format.
func MarshalJWK(bytes []byte) (JWK, error) {
	// Parse the public key and validate type.
	pub, err := keys.ParsePublicKey(bytes)
	if err != nil {
		return JWK{}, trace.Wrap(err)
	}

	switch p := pub.(type) {
	case *rsa.PublicKey:
		return marshalRSAJWK(p), nil
	case *ecdsa.PublicKey:
		return marshalECDSAJWK(p)
	default:
		return JWK{}, trace.BadParameter("unsupported public type type %T", pub)
	}
}

func marshalRSAJWK(pub *rsa.PublicKey) JWK {
	return JWK{
		KeyType:   keyTypeRSA,
		Use:       defaults.JWTUse,
		KeyID:     rsaKeyID(pub),
		Algorithm: string(jose.RS256),
		N:         base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		E:         base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}
}

func marshalECDSAJWK(pub *ecdsa.PublicKey) (JWK, error) {
	if pub.Curve != elliptic.P256() {
		return JWK{}, trace.BadParameter("unsupported curve %T", pub.Curve)
	}
	keyID, err := genericKeyID(pub)
	if err != nil {
		return JWK{}, trace.Wrap(err)
	}
	return JWK{
		KeyType:   keyTypeEC,
		Use:       defaults.JWTUse,
		KeyID:     keyID,
		Algorithm: string(jose.ES256),
		Curve:     pub.Curve.Params().Name,
		X:         base64.RawURLEncoding.EncodeToString(pub.X.Bytes()),
		Y:         base64.RawURLEncoding.EncodeToString(pub.Y.Bytes()),
	}, nil
}

// UnmarshalJWK will unmarshal JWK into a crypto.PublicKey that can be used
// to validate signatures.
func UnmarshalJWK(jwk JWK) (crypto.PublicKey, error) {
	switch jwk.KeyType {
	case keyTypeRSA:
		return unmarshalRSAJWK(jwk)
	case keyTypeEC:
		return unmarshalECDSAJWK(jwk)
	default:
		return nil, trace.BadParameter("unsupported key type %v", jwk.KeyType)
	}
}

func unmarshalRSAJWK(jwk JWK) (*rsa.PublicKey, error) {
	if jwk.Algorithm != string(jose.RS256) {
		return nil, trace.BadParameter("unsupported algorithm %v", jwk.Algorithm)
	}

	n, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	e, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(n),
		E: int(new(big.Int).SetBytes(e).Uint64()),
	}, nil
}

func unmarshalECDSAJWK(jwk JWK) (*ecdsa.PublicKey, error) {
	if jwk.Algorithm != string(jose.ES256) {
		return nil, trace.BadParameter("unsupported algorithm %v", jwk.Algorithm)
	}
	if jwk.Curve != elliptic.P256().Params().Name {
		return nil, trace.BadParameter("unsupported curve %v", jwk.Curve)
	}

	x, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	y, err := base64.RawURLEncoding.DecodeString(jwk.Y)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(x),
		Y:     new(big.Int).SetBytes(y),
	}, nil
}
