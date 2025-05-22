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
	"errors"
	"iter"
	"log/slog"
	"slices"

	"filippo.io/age"
	"github.com/gravitational/trace"

	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

// EncryptionKeyStore provides methods for interacting with encryption keys.
type EncryptionKeyStore interface {
	NewEncryptionKeyPair(ctx context.Context, purpose cryptosuites.KeyPurpose) (*types.EncryptionKeyPair, error)
	GetDecrypter(ctx context.Context, keyPair *types.EncryptionKeyPair) (crypto.Decrypter, error)
}

// ManagerConfig captures all of the dependencies required to instantiate a Manager.
type ManagerConfig struct {
	Backend  services.RecordingEncryption
	KeyStore EncryptionKeyStore
	Logger   *slog.Logger
}

// NewManager returns a new Manager using the given ManagerConfig.
func NewManager(cfg ManagerConfig) (*Manager, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	if cfg.Backend == nil {
		return nil, trace.BadParameter("backend is required")
	}

	if cfg.KeyStore == nil {
		return nil, trace.BadParameter("key store is required")
	}

	return &Manager{
		RecordingEncryption: cfg.Backend,
		keyStore:            cfg.KeyStore,
		logger:              cfg.Logger,
	}, nil
}

// A Manager wraps a services.RecordingEncryption and EncryptionKeyStore in order to provide more complex operations
// than the CRUD methods exposed by services.RecordingEncryption. It primarily handles resolving RecordingEncryption
// state and searching for accessible decryption keys.
type Manager struct {
	services.RecordingEncryption

	logger   *slog.Logger
	keyStore EncryptionKeyStore
	uploader events.MultipartUploader
}

// ensureActiveRecordingEncryption returns the configured RecordingEncryption resource if it exists with active keys. If it does not,
// then the resource will be created or updated with a new active keypair. The bool return value indicates whether or not
// a new pair was provisioned.
func (m *Manager) ensureActiveRecordingEncryption(ctx context.Context) (*recordingencryptionv1.RecordingEncryption, bool, error) {
	upsert := m.UpdateRecordingEncryption
	encryption, err := m.GetRecordingEncryption(ctx)
	if err != nil {
		if !trace.IsNotFound(err) {
			return encryption, false, trace.Wrap(err)
		}
		encryption = &recordingencryptionv1.RecordingEncryption{
			Spec: &recordingencryptionv1.RecordingEncryptionSpec{
				ActiveKeys: nil,
			},
		}
		upsert = m.CreateRecordingEncryption
	}

	activeKeys := encryption.GetSpec().ActiveKeys

	// no keys present, need to generate the initial active keypair
	if len(activeKeys) == 0 {
		m.logger.InfoContext(ctx, "no active keys, generating initial recording encryption pair")
		wrappingPair, err := m.keyStore.NewEncryptionKeyPair(ctx, cryptosuites.RecordingKeyWrapping)
		if err != nil {
			return encryption, false, trace.Wrap(err, "generating wrapping key")
		}

		ident, err := age.GenerateX25519Identity()
		if err != nil {
			return encryption, false, trace.Wrap(err, "generating age encryption key")
		}

		encryptedIdent, err := wrappingPair.EncryptOAEP([]byte(ident.String()))
		if err != nil {
			return encryption, false, trace.Wrap(err, "wrapping encryption key")
		}

		wrappedKey := recordingencryptionv1.WrappedKey{
			KeyEncryptionPair: wrappingPair,
			RecordingEncryptionPair: &types.EncryptionKeyPair{
				PrivateKeyType: types.PrivateKeyType_RAW,
				PrivateKey:     encryptedIdent,
				PublicKey:      []byte(ident.Recipient().String()),
			},
		}
		encryption.Spec.ActiveKeys = []*recordingencryptionv1.WrappedKey{&wrappedKey}
		encryption, err = upsert(ctx, encryption)
		if err != nil {
			return encryption, false, trace.Wrap(err)
		}
		return encryption, true, nil
	}

	return encryption, false, nil
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
// next state on behalf of the current auth server. At a high level it will provision unfulfilled keys for itself,
// fulfill keys for keystore configurations when possible, and move its own keys through rotation states.
func (m *Manager) ResolveRecordingEncryption(ctx context.Context) (*recordingencryptionv1.RecordingEncryption, error) {
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

// GetAgeEncryptionKeys returns an iterator of AgeEncryptionKeys from a list of WrappedKeys. This is for use in
// populating the EncryptionKeys field of SessionRecordingConfigStatus.
func GetAgeEncryptionKeys(keys []*recordingencryptionv1.WrappedKey) iter.Seq[*types.AgeEncryptionKey] {
	return func(yield func(*types.AgeEncryptionKey) bool) {
		for _, key := range keys {
			if !yield(&types.AgeEncryptionKey{
				PublicKey: key.RecordingEncryptionPair.PublicKey,
			}) {
				return
			}
		}
	}
}

// RecordingEncryptionResolver resolves RecordingEncryption state
type RecordingEncryptionResolver interface {
	ResolveRecordingEncryption(ctx context.Context) (*recordingencryptionv1.RecordingEncryption, error)
}

// WatchConfig captures required dependencies for building a RecordingEncyprtion watcher that
// automatically resolves state.
type WatchConfig struct {
	Events        types.Events
	Resolver      RecordingEncryptionResolver
	ClusterConfig services.ClusterConfiguration
	Logger        *slog.Logger
	LockConfig    backend.RunWhileLockedConfig
}

// Watch creates a watcher responsible for responding to changes in the RecordingEncryption
// resource. This is how auth servers cooperate and ensure there are accessible wrapped keys for each unique
// keystore configuration in a cluster.
func Watch(ctx context.Context, cfg WatchConfig) error {
	switch {
	case cfg.Events == nil:
		return trace.BadParameter("events is required")
	case cfg.Resolver == nil:
		return trace.BadParameter("recording encryption resolver is required")
	case cfg.ClusterConfig == nil:
		return trace.BadParameter("cluster config backend is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	cfg.Logger.DebugContext(ctx, "creating recording_encryption watcher")
	w, err := cfg.Events.NewWatcher(ctx, types.Watch{
		Name: "recording_encryption_watcher",
		Kinds: []types.WatchKind{
			{
				Kind: types.KindRecordingEncryption,
			},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	go func() {
		for {
			select {
			case ev := <-w.Events():
				if ev.Type != types.OpPut {
					continue
				}
				const retries = 3
				for tries := range retries {
					err := handleRecordingEncryptionChange(ctx, cfg)
					if err == nil {
						break
					}

					cfg.Logger.ErrorContext(ctx, "failed to handle session recording config change", "error", err, "remaining_tries", retries-tries-1)
				}

			case <-w.Done():
				cfg.Logger.DebugContext(ctx, "no longer watching recording_encryption")
				return
			}
		}
	}()

	return nil
}

// this helper handles reacting to individual Put events on the RecordingEncryption resource and updates the
// SessionRecordingConfig with the results, if necessary
func handleRecordingEncryptionChange(ctx context.Context, cfg WatchConfig) error {
	return trace.Wrap(backend.RunWhileLocked(ctx, cfg.LockConfig, func(ctx context.Context) error {
		recConfig, err := cfg.ClusterConfig.GetSessionRecordingConfig(ctx)
		if err != nil {
			return trace.Wrap(err, "fetching recording config")
		}

		if !recConfig.GetEncrypted() {
			cfg.Logger.DebugContext(ctx, "session recording encryption disabled, skip resolving keys")
			return nil
		}

		encryption, err := cfg.Resolver.ResolveRecordingEncryption(ctx)
		if err != nil {
			cfg.Logger.ErrorContext(ctx, "failed to resolve recording encryption state", "error", err)
			return trace.Wrap(err, "resolving recording encryption")
		}

		if recConfig.SetEncryptionKeys(GetAgeEncryptionKeys(encryption.GetSpec().ActiveKeys)) {
			_, err = cfg.ClusterConfig.UpdateSessionRecordingConfig(ctx, recConfig)
			return trace.Wrap(err, "updating encryption keys")
		}

		return nil
	}))
}
