/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package credentialencryption

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"

	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

// loadedKey is the credential encryption key loaded from the backend.
type loadedKey struct {
	keyPair     *types.EncryptionKeyPair
	publicKey   *rsa.PublicKey
	fingerprint string
}

func (m *Manager) getOrCreateKey(ctx context.Context) (*loadedKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.key != nil {
		return m.key, nil
	}

	resource, err := m.cfg.Backend.Get(ctx)
	switch {
	case trace.IsNotFound(err):
		resource, err = m.createKey(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case err != nil:
		return nil, trace.Wrap(err)
	}

	key, err := m.loadKey(ctx, resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	m.key = key
	return key, nil
}

func (m *Manager) createKey(ctx context.Context) (*pb.CredentialEncryptionKey, error) {
	var resource *pb.CredentialEncryptionKey
	// Hold a lock to avoid creating extra keystore keys (e.g. KMS keys), which
	// are never cleaned up.
	err := backend.RunWhileLocked(ctx, m.cfg.LockConfig, func(ctx context.Context) error {
		existing, err := m.cfg.Backend.Get(ctx)
		switch {
		case err == nil:
			resource = existing
			return nil
		case !trace.IsNotFound(err):
			return trace.Wrap(err)
		}

		keyPair, err := m.cfg.KeyStore.NewEncryptionKeyPair(ctx, cryptosuites.UserCredentialKeyWrapping)
		if err != nil {
			return trace.Wrap(err)
		}
		resource, err = m.cfg.Backend.Create(ctx, pb.CredentialEncryptionKey_builder{
			Kind:    types.KindCredentialEncryptionKey,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: types.MetaNameCredentialEncryptionKey,
			}.Build(),
			Spec: pb.CredentialEncryptionKeySpec_builder{
				PrivateKey:     keyPair.PrivateKey,
				PrivateKeyType: keyPair.PrivateKeyType.String(),
			}.Build(),
		}.Build())
		return trace.Wrap(err)
	})
	return resource, trace.Wrap(err)
}

func (m *Manager) loadKey(ctx context.Context, resource *pb.CredentialEncryptionKey) (*loadedKey, error) {
	privateKeyType, ok := types.PrivateKeyType_value[resource.GetSpec().GetPrivateKeyType()]
	if !ok {
		return nil, trace.BadParameter("invalid private key type %q", resource.GetSpec().GetPrivateKeyType())
	}

	keyPair := &types.EncryptionKeyPair{
		PrivateKey:     resource.GetSpec().GetPrivateKey(),
		PrivateKeyType: types.PrivateKeyType(privateKeyType),
		// The key is always used with RSA-OAEP SHA-256.
		Hash: uint32(crypto.SHA256),
	}
	pub, err := m.cfg.KeyStore.GetEncryptionPublicKey(ctx, keyPair.PrivateKey, keyPair.PrivateKeyType)
	if err != nil {
		return nil, trace.Wrap(err, "deriving credential encryption public key")
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, trace.BadParameter("expected RSA public key for credential encryption, got %T", pub)
	}
	// GetDecrypter requires the PKIX public key on the key pair.
	keyPair.PublicKey, err = x509.MarshalPKIXPublicKey(rsaPub)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fingerprint := sha256.Sum256(keyPair.PublicKey)
	return &loadedKey{
		keyPair:     keyPair,
		publicKey:   rsaPub,
		fingerprint: hex.EncodeToString(fingerprint[:]),
	}, nil
}
