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
	"bytes"
	"context"
	"crypto"
	"encoding/json"
	"io"
	"sync"

	"filippo.io/age"
	"github.com/gravitational/trace"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

// KeyStore is the subset of the Auth keystore used for credential encryption.
type KeyStore interface {
	// NewEncryptionKeyPair creates a new encryption key pair.
	NewEncryptionKeyPair(ctx context.Context, purpose cryptosuites.KeyPurpose) (*types.EncryptionKeyPair, error)
	// GetEncryptionPublicKey retrieves the public key for the given private
	// key from the keystore.
	GetEncryptionPublicKey(ctx context.Context, privateKey []byte, privateKeyType types.PrivateKeyType) (crypto.PublicKey, error)
	// GetDecrypter returns the decrypter for the given key pair.
	GetDecrypter(ctx context.Context, keyPair *types.EncryptionKeyPair) (crypto.Decrypter, error)
}

// Backend stores the cluster's credential encryption key resource.
type Backend interface {
	// Get returns the credential encryption key.
	Get(ctx context.Context) (*pb.CredentialEncryptionKey, error)
	// Create creates the credential encryption key. Returns AlreadyExists if
	// the key already exists.
	Create(ctx context.Context, resource *pb.CredentialEncryptionKey) (*pb.CredentialEncryptionKey, error)
}

// Payload is the envelope encrypted by the Auth-layer. It binds the
// credential to its owner and the target resource.
type Payload struct {
	// User is the Teleport username that owns the credential.
	User string `json:"user"`
	// ResourceKind is the kind of the resource the credential is for (e.g.
	// "git_server").
	ResourceKind string `json:"resource_kind"`
	// ResourceName is the name of the resource the credential is for.
	ResourceName string `json:"resource_name"`
	// Credential is the plaintext credential (e.g. an OAuth access token).
	Credential []byte `json:"credential"`
}

func (p *Payload) check() error {
	switch {
	case p.User == "":
		return trace.BadParameter("missing user")
	case p.ResourceKind == "":
		return trace.BadParameter("missing resource kind")
	case p.ResourceName == "":
		return trace.BadParameter("missing resource name")
	case len(p.Credential) == 0:
		return trace.BadParameter("missing credential")
	}
	return nil
}

// ManagerConfig captures all of the dependencies required to instantiate a
// Manager.
type ManagerConfig struct {
	// KeyStore is the Auth keystore.
	KeyStore KeyStore
	// Backend stores the credential encryption key resource.
	Backend Backend
	// LockConfig is the configuration of the lock held while creating the
	// credential encryption key, so only one Auth server creates the key.
	LockConfig backend.RunWhileLockedConfig
	// FIPS means FedRAMP/FIPS compliant configuration was requested.
	FIPS bool
}

// Manager encrypts and decrypts payloads with the cluster's credential
// encryption key. The key is created on first use.
type Manager struct {
	cfg ManagerConfig

	mu  sync.Mutex
	key *loadedKey
}

// NewManager returns a new Manager using the given [ManagerConfig].
func NewManager(cfg ManagerConfig) (*Manager, error) {
	switch {
	case cfg.KeyStore == nil:
		return nil, trace.BadParameter("key store is required")
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend is required")
	}
	if err := cfg.LockConfig.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err, "invalid lock config")
	}
	return &Manager{
		cfg: cfg,
	}, nil
}

// EncryptPayload encrypts the payload envelope with the credential encryption
// key.
func (m *Manager) EncryptPayload(ctx context.Context, payload *Payload) ([]byte, error) {
	if err := payload.check(); err != nil {
		return nil, trace.Wrap(err)
	}
	plaintext, err := json.Marshal(payload)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ciphertext, err := m.encrypt(ctx, plaintext)
	return ciphertext, trace.Wrap(err)
}

// DecryptPayload decrypts a payload envelope encrypted by EncryptPayload.
func (m *Manager) DecryptPayload(ctx context.Context, ciphertext []byte) (*Payload, error) {
	plaintext, err := m.decrypt(ctx, ciphertext)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var payload Payload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := payload.check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &payload, nil
}

func (m *Manager) encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	// age encrypts payloads with ChaCha20-Poly1305, which is not FIPS approved.
	if m.cfg.FIPS {
		return nil, trace.BadParameter("user external credentials are not supported in FIPS mode")
	}
	key, err := m.getOrCreateKey(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, &recipient{
		publicKey:   key.publicKey,
		fingerprint: key.fingerprint,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if _, err := w.Write(plaintext); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := w.Close(); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func (m *Manager) decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	if m.cfg.FIPS {
		return nil, trace.BadParameter("user external credentials are not supported in FIPS mode")
	}
	key, err := m.getOrCreateKey(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	r, err := age.Decrypt(bytes.NewReader(ciphertext), &identity{
		ctx:      ctx,
		keyStore: m.cfg.KeyStore,
		key:      key,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	plaintext, err := io.ReadAll(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return plaintext, nil
}
