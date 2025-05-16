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
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
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
	"github.com/gravitational/teleport/lib/session"
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
		cfg.Logger = slog.New(slog.DiscardHandler)
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

// ResolveRecordingEncryption examines the current state of the RescordingEncryption resource and advances it to the
// next state on behalf of the current auth server. At a high level it will provision unfulfilled keys for itself,
// fulfill keys for keystore configurations when possible, and move its own keys through rotation states.
func (m *Manager) ResolveRecordingEncryption(ctx context.Context) (*recordingencryptionv1.RecordingEncryption, error) {
	m.logger.DebugContext(ctx, "fetching recording config")
	upsert := m.UpdateRecordingEncryption
	encryption, err := m.GetRecordingEncryption(ctx)
	if err != nil {
		if !trace.IsNotFound(err) {
			return encryption, trace.Wrap(err)
		}
		encryption = &recordingencryptionv1.RecordingEncryption{
			Spec: &recordingencryptionv1.RecordingEncryptionSpec{
				ActiveKeys: nil,
			},
		}
		upsert = m.CreateRecordingEncryption
	}

	m.logger.DebugContext(ctx, "recording encryption enabled, checking for active keys")
	activeKeys := encryption.GetSpec().ActiveKeys

	// no keys present, need to generate the initial active keypair
	if len(activeKeys) == 0 {
		m.logger.DebugContext(ctx, "no active keys, generating initial keyset")
		wrappingPair, err := m.keyStore.NewEncryptionKeyPair(ctx, cryptosuites.RecordingKeyWrapping)
		if err != nil {
			return encryption, trace.Wrap(err, "generating wrapping key")
		}

		ident, err := age.GenerateX25519Identity()
		if err != nil {
			return encryption, trace.Wrap(err, "generating age encryption key")
		}

		encryptedIdent, err := wrappingPair.EncryptOAEP([]byte(ident.String()))
		if err != nil {
			return encryption, trace.Wrap(err, "wrapping encryption key")
		}

		wrappedKey := recordingencryptionv1.WrappedKey{
			KeyEncryptionPair: wrappingPair,
			RecordingEncryptionPair: &types.EncryptionKeyPair{
				PrivateKeyType: types.PrivateKeyType_RAW,
				PrivateKey:     encryptedIdent,
				PublicKey:      []byte(ident.Recipient().String()),
			},
			State: recordingencryptionv1.KeyState_KEY_STATE_ACTIVE,
		}
		encryption.Spec.ActiveKeys = []*recordingencryptionv1.WrappedKey{&wrappedKey}
		m.logger.DebugContext(ctx, "updating session recording encryption active keys")
		encryption, err = upsert(ctx, encryption)
		return encryption, trace.Wrap(err)
	}

	m.logger.DebugContext(ctx, "searching for accessible active key")
	var activeKey *recordingencryptionv1.WrappedKey
	var decrypter crypto.Decrypter
	var unfulfilledKeys []*recordingencryptionv1.WrappedKey
	var ownUnfulfilledKey bool
	rotatingKeys := make(map[*recordingencryptionv1.WrappedKey]struct{})
	for _, key := range activeKeys {
		if key.RecordingEncryptionPair == nil {
			unfulfilledKeys = append(unfulfilledKeys, key)
		}

		dec, err := m.keyStore.GetDecrypter(ctx, key.KeyEncryptionPair)
		if err != nil {
			continue
		}

		// if we make it to this section the key is accessible to the current auth server
		if key.RecordingEncryptionPair == nil {
			ownUnfulfilledKey = true
			continue
		}

		activeKey = key
		decrypter = dec
		if key.State == recordingencryptionv1.KeyState_KEY_STATE_ROTATING {
			rotatingKeys[key] = struct{}{}
		}
	}

	// create unfulfilled key if necessary
	if activeKey == nil || activeKey.State == recordingencryptionv1.KeyState_KEY_STATE_ROTATING {
		if ownUnfulfilledKey {
			m.logger.DebugContext(ctx, "waiting for key fulfillment, nothing more to do")
			return encryption, nil
		}

		m.logger.DebugContext(ctx, "no accessible keys, generating empty key to be fulfilled")
		keypair, err := m.keyStore.NewEncryptionKeyPair(ctx, cryptosuites.RecordingKeyWrapping)
		if err != nil {
			return encryption, trace.Wrap(err, "generating keypair for new wrapped key")
		}
		activeKeys = append(activeKeys, &recordingencryptionv1.WrappedKey{
			KeyEncryptionPair: keypair,
			State:             recordingencryptionv1.KeyState_KEY_STATE_ACTIVE,
		})

		encryption.Spec.ActiveKeys = activeKeys
		encryption, err = m.UpdateRecordingEncryption(ctx, encryption)
		return encryption, trace.Wrap(err, "updating session recording config")
	}

	var shouldUpdate bool
	m.logger.DebugContext(ctx, "active key is accessible, fulfilling empty keys", "keys_waiting", len(unfulfilledKeys))
	if len(unfulfilledKeys) > 0 {
		decryptionKey, err := decrypter.Decrypt(rand.Reader, activeKey.RecordingEncryptionPair.PrivateKey, nil)
		if err != nil {
			return encryption, trace.Wrap(err, "decrypting known key")
		}

		for _, key := range unfulfilledKeys {
			encryptedKey, err := key.KeyEncryptionPair.EncryptOAEP(decryptionKey)
			if err != nil {
				return encryption, trace.Wrap(err, "reencrypting decryption key")
			}

			key.RecordingEncryptionPair = &types.EncryptionKeyPair{
				PrivateKey: encryptedKey,
				PublicKey:  activeKey.RecordingEncryptionPair.PublicKey,
			}
			key.State = recordingencryptionv1.KeyState_KEY_STATE_ACTIVE

			shouldUpdate = true
		}
	}

	if len(rotatingKeys) > 0 {
		m.logger.DebugContext(ctx, "marking rotated keys", "key_count", len(rotatingKeys))
	}
	for _, key := range activeKeys {
		if _, ok := rotatingKeys[key]; ok {
			key.State = recordingencryptionv1.KeyState_KEY_STATE_ROTATED
			continue
		}
	}

	if shouldUpdate {
		m.logger.DebugContext(ctx, "updating recording_encryption resource")
		encryption, err = m.UpdateRecordingEncryption(ctx, encryption)
		if err != nil {
			return encryption, trace.Wrap(err, "updating session recording config")
		}
	}

	return encryption, nil
}

func (m *Manager) searchActiveKeys(ctx context.Context, activeKeys []*recordingencryptionv1.WrappedKey, publicKey []byte) (*types.EncryptionKeyPair, error) {
	for _, key := range activeKeys {
		if key.GetRecordingEncryptionPair() == nil {
			continue
		}

		// TODO (eriktate): this is a bit of a hack to allow encryption to work while the public key isn't retrievable
		// from the age header
		if publicKey != nil {
			if !slices.Equal(key.RecordingEncryptionPair.PublicKey, publicKey) {
				continue
			}
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

	return nil, trace.NotFound("no accessible decryption key found")
}

// FindDecryptionKey returns the first accessible decryption key that matches one of the given public keys.
func (m *Manager) FindDecryptionKey(ctx context.Context, publicKeys ...[]byte) (*types.EncryptionKeyPair, error) {
	encryption, err := m.GetRecordingEncryption(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO (eriktate): search rotated keys as well once rotation is implemented
	activeKeys := encryption.GetSpec().GetActiveKeys()
	if len(publicKeys) == 0 {
		return m.searchActiveKeys(ctx, activeKeys, nil)
	}

	for _, publicKey := range publicKeys {
		found, err := m.searchActiveKeys(ctx, activeKeys, publicKey)
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}

			return nil, trace.Wrap(err)
		}

		return found, nil
	}

	return nil, trace.NotFound("no accessible decryption key found")
}

func (m *Manager) UploadEncryptedRecording(ctx context.Context) (chan *recordingencryptionv1.UploadEncryptedRecordingRequest, chan error) {
	inputCh := make(chan *recordingencryptionv1.UploadEncryptedRecordingRequest)
	errCh := make(chan error)

	go func() (err error) {
		defer func() {
			errCh <- err
		}()

		var upload *events.StreamUpload
		var parts []events.StreamPart
		var req *recordingencryptionv1.UploadEncryptedRecordingRequest
		moreParts := true
		for moreParts {
			if err := m.uploader.ReserveUploadPart(ctx, *upload, req.PartIndex+1); err != nil {
				return trace.Wrap(err)
			}

			select {
			case req, moreParts = <-inputCh:
				if !moreParts {
					break
				}

				if upload == nil {
					sessID, err := session.ParseID(req.SessionId)
					if err != nil {
						return trace.Wrap(err)
					}

					upload, err = m.uploader.CreateUpload(ctx, *sessID)
					if err != nil {
						return trace.Wrap(err)
					}
					continue
				}

				part, err := m.uploader.UploadPart(ctx, *upload, req.PartIndex, bytes.NewReader(req.Part))
				if err != nil {
					return trace.Wrap(err)
				}
				parts = append(parts, *part)

			case <-ctx.Done():
				return trace.Wrap(ctx.Err())
			}
		}
		return trace.Wrap(m.uploader.CompleteUpload(ctx, *upload, parts))
	}()

	return inputCh, errCh
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

// RecordingEncryptionWatchConfig captures required dependencies for building a RecordingEncyprtion watcher that
// automatically resolves state.
type RecordingEncryptionWatchConfig struct {
	Events        types.Events
	Resolver      RecordingEncryptionResolver
	ClusterConfig services.ClusterConfiguration
	Logger        *slog.Logger
	LockConfig    backend.RunWhileLockedConfig
}

// Watch creates a watcher responsible for responding to changes in the RecordingEncryption
// resource. This is how auth servers cooperate and ensure there are accessible wrapped keys for each unique
// keystore configuration in a cluster.
func Watch(ctx context.Context, cfg RecordingEncryptionWatchConfig) error {
	switch {
	case cfg.Events == nil:
		return trace.BadParameter("events is required")
	case cfg.Resolver == nil:
		return trace.BadParameter("recording encryption resolver is required")
	case cfg.ClusterConfig == nil:
		return trace.BadParameter("cluster config backend is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
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
func handleRecordingEncryptionChange(ctx context.Context, cfg RecordingEncryptionWatchConfig) error {
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
