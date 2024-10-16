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

package keystore

import (
	"context"
	"crypto"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

type softwareKeyStore struct {
	rsaKeyPairSource RSAKeyPairSource
}

// RSAKeyPairSource is a function type which returns new RSA keypairs.
type RSAKeyPairSource func() (priv []byte, pub []byte, err error)

type softwareConfig struct {
	rsaKeyPairSource RSAKeyPairSource
}

func newSoftwareKeyStore(config *softwareConfig) *softwareKeyStore {
	return &softwareKeyStore{
		rsaKeyPairSource: config.rsaKeyPairSource,
	}
}

func (p *softwareKeyStore) name() string {
	return storeSoftware
}

// keyTypeDescription returns a human-readable description of the types of keys
// this backend uses.
func (s *softwareKeyStore) keyTypeDescription() string {
	return "raw software keys"
}

// generateRSA creates a new private key and returns its identifier and a crypto.Signer. The returned
// identifier for softwareKeyStore is a pem-encoded private key, and can be passed to getSigner later to get
// an equivalent crypto.Signer.
func (s *softwareKeyStore) generateKey(ctx context.Context, alg cryptosuites.Algorithm) ([]byte, crypto.Signer, error) {
	if alg == cryptosuites.RSA2048 && s.rsaKeyPairSource != nil {
		privateKeyPEM, _, err := s.rsaKeyPairSource()
		if err != nil {
			return nil, nil, err
		}
		signer, err := keys.ParsePrivateKey(privateKeyPEM)
		return privateKeyPEM, signer, trace.Wrap(err)
	}
	signer, err := cryptosuites.GenerateKeyWithAlgorithm(alg)
	if err != nil {
		return nil, nil, err
	}
	privateKeyPEM, err := keys.MarshalPrivateKey(signer)
	return privateKeyPEM, signer, trace.Wrap(err)
}

// getSigner returns a crypto.Signer for the given pem-encoded private key.
func (s *softwareKeyStore) getSigner(ctx context.Context, rawKey []byte, publicKey crypto.PublicKey) (crypto.Signer, error) {
	return keys.ParsePrivateKey(rawKey)
}

// canSignWithKey returns true if the given key is a raw key.
func (s *softwareKeyStore) canSignWithKey(ctx context.Context, _ []byte, keyType types.PrivateKeyType) (bool, error) {
	return keyType == types.PrivateKeyType_RAW, nil
}

// deleteKey is a no-op for softwareKeyStore because the keys are not actually
// stored in any external backend.
func (s *softwareKeyStore) deleteKey(_ context.Context, _ []byte) error {
	return nil
}

// deleteUnusedKeys is a no-op for softwareKeyStore because the keys are not
// actually stored in any external backend.
func (s *softwareKeyStore) deleteUnusedKeys(ctx context.Context, activeKeys [][]byte) error {
	return nil
}
