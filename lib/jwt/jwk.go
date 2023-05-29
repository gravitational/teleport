/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
