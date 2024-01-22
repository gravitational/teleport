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
	"crypto/rsa"
	"encoding/base64"
	"math/big"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

// JWK is a JSON Web Key, described in detail in RFC 7517.
type JWK struct {
	// KeyType is the type of asymmetric key used.
	KeyType string `json:"kty"`
	// Algorithm used to sign.
	Algorithm string `json:"alg"`
	// N is the modulus of the public key.
	N string `json:"n"`
	// E is the exponent of the public key.
	E string `json:"e"`
	// Use identifies the intended use of the public key.
	// This field is required for the AWS OIDC Integration.
	// https://www.rfc-editor.org/rfc/rfc7517#section-4.2
	Use string `json:"use"`
	// KeyID identifies the key to use.
	// This field is required (even if empty) for the AWS OIDC Integration.
	// https://www.rfc-editor.org/rfc/rfc7517#section-4.5
	KeyID string `json:"kid"`
}

// MarshalJWK will marshal a supported public key into JWK format.
func MarshalJWK(bytes []byte) (JWK, error) {
	// Parse the public key and validate type.
	p, err := utils.ParsePublicKey(bytes)
	if err != nil {
		return JWK{}, trace.Wrap(err)
	}
	publicKey, ok := p.(*rsa.PublicKey)
	if !ok {
		return JWK{}, trace.BadParameter("unsupported key format %T", p)
	}

	// Marshal to JWK.
	return JWK{
		KeyType:   string(defaults.ApplicationTokenKeyType),
		Algorithm: string(defaults.ApplicationTokenAlgorithm),
		N:         base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes()),
		E:         base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes()),
		Use:       defaults.JWTUse,
		KeyID:     "",
	}, nil
}

// UnmarshalJWK will unmarshal JWK into a crypto.PublicKey that can be used
// to validate signatures.
func UnmarshalJWK(jwk JWK) (crypto.PublicKey, error) {
	if jwk.KeyType != string(defaults.ApplicationTokenKeyType) {
		return nil, trace.BadParameter("unsupported key type %v", jwk.KeyType)
	}
	if jwk.Algorithm != string(defaults.ApplicationTokenAlgorithm) {
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
