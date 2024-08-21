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
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/utils"
)

type softwareKeyStore struct {
	rsaKeyPairSource RSAKeyPairSource
}

// RSAKeyPairSource is a function type which returns new RSA keypairs.
type RSAKeyPairSource func() (priv []byte, pub []byte, err error)

type SoftwareConfig struct {
	RSAKeyPairSource RSAKeyPairSource
}

func (cfg *SoftwareConfig) CheckAndSetDefaults() error {
	if cfg.RSAKeyPairSource == nil {
		cfg.RSAKeyPairSource = native.GenerateKeyPair
	}
	return nil
}

func newSoftwareKeyStore(config *SoftwareConfig, logger logrus.FieldLogger) *softwareKeyStore {
	return &softwareKeyStore{
		rsaKeyPairSource: config.RSAKeyPairSource,
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

// generateRSA creates a new RSA private key and returns its identifier and a
// crypto.Signer. The returned identifier for softwareKeyStore is a pem-encoded
// private key, and can be passed to getSigner later to get the same
// crypto.Signer.
func (s *softwareKeyStore) generateRSA(ctx context.Context, _ ...RSAKeyOption) ([]byte, crypto.Signer, error) {
	priv, _, err := s.rsaKeyPairSource()
	if err != nil {
		return nil, nil, err
	}
	signer, err := s.getSignerWithoutPublicKey(ctx, priv)
	if err != nil {
		return nil, nil, err
	}
	return priv, signer, trace.Wrap(err)
}

// getSigner returns a crypto.Signer for the given pem-encoded private key.
func (s *softwareKeyStore) getSigner(ctx context.Context, rawKey []byte, publicKey crypto.PublicKey) (crypto.Signer, error) {
	return s.getSignerWithoutPublicKey(ctx, rawKey)
}

func (s *softwareKeyStore) getSignerWithoutPublicKey(ctx context.Context, rawKey []byte) (crypto.Signer, error) {
	signer, err := utils.ParsePrivateKeyPEM(rawKey)
	return signer, trace.Wrap(err)
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
