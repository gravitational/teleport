// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package recordingencryption

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"slices"

	"filippo.io/age"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/services"
)

// KeyStore provides methods for interacting with encryption keys.
type KeyStore interface {
	NewEncryptionKeyPair(ctx context.Context, purpose cryptosuites.KeyPurpose) (*types.EncryptionKeyPair, error)
	GetDecrypter(ctx context.Context, keyPair *types.EncryptionKeyPair) (crypto.Decrypter, error)
}

// ManagerConfig captures all of the dependencies required to instantiate a Manager.
type ManagerConfig struct {
	Backend    services.RecordingEncryption
	KeyStore   KeyStore
	Logger     *slog.Logger
	LockConfig backend.RunWhileLockedConfig
}

// NewManager returns a new Manager using the given ManagerConfig.
func NewManager(cfg ManagerConfig) (*Manager, error) {
	if cfg.Backend == nil {
		return nil, trace.BadParameter("backend is required")
	}

	if cfg.KeyStore == nil {
		return nil, trace.BadParameter("key store is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, "recording-encryption-manager")
	}

	return &Manager{
		RecordingEncryption: cfg.Backend,
		keyStore:            cfg.KeyStore,
		lockConfig:          cfg.LockConfig,
		logger:              cfg.Logger,
	}, nil
}

// A Manager wraps a services.RecordingEncryption and KeyStore in order to provide more complex operations
// than the CRUD methods exposed by services.RecordingEncryption. It primarily handles resolving RecordingEncryption
// state and searching for accessible decryption keys.
type Manager struct {
	services.RecordingEncryption

	logger     *slog.Logger
	lockConfig backend.RunWhileLockedConfig
	keyStore   KeyStore
}

// ensureActiveRecordingEncryption returns the configured RecordingEncryption resource if it exists with active keys. If it does not,
// then the resource will be created or updated with a new active keypair. The bool return value indicates whether or not
// a new pair was provisioned.
func (m *Manager) ensureActiveRecordingEncryption(ctx context.Context) (*recordingencryptionv1.RecordingEncryption, bool, error) {
	persistFn := m.UpdateRecordingEncryption
	encryption, err := m.GetRecordingEncryption(ctx)
	if err != nil {
		if !trace.IsNotFound(err) {
			return encryption, false, trace.Wrap(err)
		}
		encryption = &recordingencryptionv1.RecordingEncryption{
			Spec: &recordingencryptionv1.RecordingEncryptionSpec{},
		}
		persistFn = m.CreateRecordingEncryption
	}

	activeKeys := encryption.GetSpec().ActiveKeys

	// no keys present, need to generate the initial active keypair
	if len(activeKeys) > 0 {
		return encryption, false, nil
	}

	keyEncryptionPair, err := m.keyStore.NewEncryptionKeyPair(ctx, cryptosuites.RecordingKeyWrapping)
	if err != nil {
		return encryption, false, trace.Wrap(err, "generating wrapping key")
	}

	ident, err := age.GenerateX25519Identity()
	if err != nil {
		return encryption, false, trace.Wrap(err, "generating age encryption key")
	}

	encryptedIdent, err := keyEncryptionPair.EncryptOAEP([]byte(ident.String()))
	if err != nil {
		return encryption, false, trace.Wrap(err, "wrapping encryption key")
	}

	wrappedKey := recordingencryptionv1.WrappedKey{
		KeyEncryptionPair: keyEncryptionPair,
		RecordingEncryptionPair: &types.EncryptionKeyPair{
			PrivateKeyType: types.PrivateKeyType_RAW,
			PrivateKey:     encryptedIdent,
			PublicKey:      []byte(ident.Recipient().String()),
		},
	}
	encryption.Spec.ActiveKeys = []*recordingencryptionv1.WrappedKey{&wrappedKey}
	encryption, err = persistFn(ctx, encryption)
	if err != nil {
		return encryption, false, trace.Wrap(err)
	}
	fp := sha256.Sum256(wrappedKey.RecordingEncryptionPair.PublicKey)
	m.logger.InfoContext(ctx, "no active keys, generated initial recording encryption pair", "public_fingerprint", hex.EncodeToString(fp[:]))
	return encryption, true, nil
}

var errWaitingForKey = errors.New("waiting for key to be fulfilled")

// getRecordingEncryptionKey returns the first active recording encryption key accessible to the configured key store.
func (m *Manager) getRecordingEncryptionKeyPair(ctx context.Context, keys []*recordingencryptionv1.WrappedKey) (*types.EncryptionKeyPair, error) {
	var foundUnfulfilledKey bool
	for _, key := range keys {
		decrypter, err := m.keyStore.GetDecrypter(ctx, key.KeyEncryptionPair)
		if err != nil {
			continue
		}

		// if we make it to this section the key is accessible to the current auth server
		if key.RecordingEncryptionPair == nil {
			foundUnfulfilledKey = true
			continue
		}

		decryptionKey, err := decrypter.Decrypt(rand.Reader, key.RecordingEncryptionPair.PrivateKey, nil)
		if err != nil {
			return nil, trace.Wrap(err, "decrypting known key")
		}

		return &types.EncryptionKeyPair{
			PrivateKey: decryptionKey,
			PublicKey:  key.RecordingEncryptionPair.PublicKey,
		}, nil
	}

	if foundUnfulfilledKey {
		return nil, trace.Wrap(errWaitingForKey)
	}

	return nil, trace.NotFound("no accessible recording encryption pair found")
}

// ResolveRecordingEncryption examines the current state of the RescordingEncryption resource and advances it to the
// next state on behalf of the current auth server.
//
// When no active recording encryption key pairs exist, the first pair will be generated and wrapped using a new key
// encryption pair generated by the Manager's keystore.
//
// When at least one active keypair exists but none are accessible to the Manager's keystore, a new key encryption pair
// will be generated and saved without a key encryption pair. This is an unfulfilled key that some other instance of
// Manager on another auth server will need to fulfill asynchronously.
//
// If at least one active key is accessible to the Manager's keystore, then unfulfilled keys (identified by missing
// recording encryption key pairs) will be fulfilled using their public key encryption keys.
//
// If there are no unfulfilled keys present, then nothing should be done.
func (m *Manager) ResolveRecordingEncryption(ctx context.Context, postProcessFn func(context.Context, *recordingencryptionv1.RecordingEncryption) error) (encryption *recordingencryptionv1.RecordingEncryption, err error) {
	err = backend.RunWhileLocked(ctx, m.lockConfig, func(ctx context.Context) error {
		encryption, err = m.resolveRecordingEncryption(ctx)
		if err != nil {
			return err
		}
		if postProcessFn != nil {
			return postProcessFn(ctx, encryption)
		}
		return nil
	})
	return encryption, trace.Wrap(err)
}

func (m *Manager) resolveRecordingEncryption(ctx context.Context) (*recordingencryptionv1.RecordingEncryption, error) {
	encryption, generatedKey, err := m.ensureActiveRecordingEncryption(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if generatedKey {
		m.logger.DebugContext(ctx, "created initial recording encryption key")
		return encryption, nil
	}

	activeKeys := encryption.GetSpec().ActiveKeys
	recordingEncryptionPair, err := m.getRecordingEncryptionKeyPair(ctx, activeKeys)
	if err != nil {
		if errors.Is(err, errWaitingForKey) {
			// do nothing
			return encryption, nil
		}

		if trace.IsNotFound(err) {
			m.logger.InfoContext(ctx, "no accessible recording encryption keys, posting new key to be fulfilled")
			keypair, err := m.keyStore.NewEncryptionKeyPair(ctx, cryptosuites.RecordingKeyWrapping)
			if err != nil {
				return nil, trace.Wrap(err, "generating keypair for new wrapped key")
			}
			encryption.GetSpec().ActiveKeys = append(activeKeys, &recordingencryptionv1.WrappedKey{
				KeyEncryptionPair: keypair,
			})

			encryption, err = m.UpdateRecordingEncryption(ctx, encryption)
			return encryption, trace.Wrap(err, "updating session recording config")
		}

		return nil, trace.Wrap(err)
	}

	var shouldUpdate bool
	for _, key := range activeKeys {
		if key.RecordingEncryptionPair != nil {
			continue
		}

		encryptedKey, err := key.KeyEncryptionPair.EncryptOAEP(recordingEncryptionPair.PrivateKey)
		if err != nil {
			return encryption, trace.Wrap(err, "reencrypting decryption key")
		}

		key.RecordingEncryptionPair = &types.EncryptionKeyPair{
			PrivateKey: encryptedKey,
			PublicKey:  recordingEncryptionPair.PublicKey,
		}

		shouldUpdate = true
	}

	if shouldUpdate {
		m.logger.DebugContext(ctx, "fulfilling empty keys")
		encryption, err = m.UpdateRecordingEncryption(ctx, encryption)
		if err != nil {
			return encryption, trace.Wrap(err, "updating session recording config")
		}
	}

	return encryption, nil
}

// FindDecryptionKey returns the first accessible decryption key that matches one of the given public keys.
func (m *Manager) FindDecryptionKey(ctx context.Context, publicKeys ...[]byte) (*types.EncryptionKeyPair, error) {
	encryption, err := m.GetRecordingEncryption(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO (eriktate): search rotated keys as well once rotation is implemented
	activeKeys := encryption.GetSpec().ActiveKeys
	for _, publicKey := range publicKeys {
		for _, key := range activeKeys {
			if key.GetRecordingEncryptionPair() == nil {
				continue
			}

			if !slices.Equal(key.RecordingEncryptionPair.PublicKey, publicKey) {
				continue
			}

			decrypter, err := m.keyStore.GetDecrypter(ctx, key.KeyEncryptionPair)
			if err != nil {
				if !trace.IsNotFound(err) {
					m.logger.ErrorContext(ctx, "could not get decrypter from key store", "error", err)
				}
				continue
			}

			privateKey, err := decrypter.Decrypt(rand.Reader, key.RecordingEncryptionPair.PrivateKey, nil)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return &types.EncryptionKeyPair{
				PrivateKey:     privateKey,
				PublicKey:      key.RecordingEncryptionPair.PublicKey,
				PrivateKeyType: key.RecordingEncryptionPair.PrivateKeyType,
			}, nil
		}
	}

	return nil, trace.NotFound("no accessible decryption key found")
}
