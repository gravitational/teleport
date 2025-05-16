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
	"iter"
	"log/slog"
	"slices"
	"time"

	"filippo.io/age"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/services"
)

// EncryptionKeyStore provides methods for interacting with encryption keys.
type EncryptionKeyStore interface {
	NewEncryptionKeyPair(ctx context.Context, purpose cryptosuites.KeyPurpose) (*types.EncryptionKeyPair, error)
	GetDecrypter(ctx context.Context, keyPair *types.EncryptionKeyPair) (crypto.Decrypter, error)
}

// A Cache fetches a cached *recordingencryptionv1.RecordingEncryption
type Cache interface {
	GetRecordingEncryption(context.Context) (*recordingencryptionv1.RecordingEncryption, error)
}

// ManagerConfig captures all of the dependencies required to instantiate a Manager.
type ManagerConfig struct {
	Backend  services.RecordingEncryption
	Cache    Cache
	KeyStore EncryptionKeyStore
	Logger   *slog.Logger
}

// NewManager returns a new Manager using the given ManagerConfig.
func NewManager(cfg ManagerConfig) (*Manager, error) {

	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend is required")
	case cfg.KeyStore == nil:
		return nil, trace.BadParameter("key store is required")
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &Manager{
		RecordingEncryption: cfg.Backend,
		keyStore:            cfg.KeyStore,
		logger:              cfg.Logger,
		cache:               cfg.Cache,
	}, nil
}

// A Manager wraps a services.RecordingEncryption and EncryptionKeyStore in order to provide more complex operations
// than the CRUD methods exposed by services.RecordingEncryption. It primarily handles resolving RecordingEncryption
// state and searching for accessible decryption keys.
type Manager struct {
	services.RecordingEncryption

	logger   *slog.Logger
	keyStore EncryptionKeyStore
	cache    Cache
}

// SetCache overwrites the configured Cache implementation
func (m *Manager) SetCache(cache Cache) {
	m.cache = cache
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
	encryption, err = upsert(ctx, encryption)
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
// next state on behalf of the current auth server. When no active recording encryption key pairs exist, the first
// pair will be generated and wrapped using a new key encryption pair generated by the Manager's keystore. When at
// least one active keypair exists but none are accessible to the Manager's keystore, a new key encryption pair will
// be generated and saved without a key encryption key pair. This is an unfulfilled key that some other instance of
// Manager on another auth server will need to fulfill asynchrnously. If at least one active key is accessible ot the
// Manager's keystore, then unfulfilled keys (identified by missing recording encryption key pairs) will be fulfilled
// using their public keys. If there are no unfulfilled keys present, then nothing should be done.
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

func (m *Manager) searchActiveKeys(ctx context.Context, activeKeys []*recordingencryptionv1.WrappedKey, publicKey []byte) (*types.EncryptionKeyPair, error) {
	for _, key := range activeKeys {
		if key.GetRecordingEncryptionPair() == nil {
			continue
		}

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
func (m *Manager) FindDecryptionKey(publicKeys ...[]byte) (*types.EncryptionKeyPair, error) {
	ctx := context.Background()
	encryption, err := m.cache.GetRecordingEncryption(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO (eriktate): search rotated keys as well once rotation is implemented
	activeKeys := encryption.GetSpec().ActiveKeys
	if len(publicKeys) == 0 {
		return m.searchActiveKeys(ctx, activeKeys, nil)
	}

	for _, publicKey := range publicKeys {
		found, err := m.searchActiveKeys(ctx, activeKeys, publicKey)
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}

			if !slices.Equal(found.PublicKey, publicKey) {
				continue
			}

			decrypter, err := m.keyStore.GetDecrypter(ctx, found)
			if err != nil {
				if !trace.IsNotFound(err) {
					m.logger.ErrorContext(ctx, "could not get decrypter from key store", "error", err)
				}
				continue
			}

			privateKey, err := decrypter.Decrypt(rand.Reader, found.PrivateKey, nil)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return &types.EncryptionKeyPair{
				PrivateKey:     privateKey,
				PublicKey:      found.PublicKey,
				PrivateKeyType: found.PrivateKeyType,
			}, nil
		}

		return found, nil
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

// Resolver resolves RecordingEncryption state
type Resolver interface {
	ResolveRecordingEncryption(ctx context.Context) (*recordingencryptionv1.RecordingEncryption, error)
}

// WatchConfig captures required dependencies for building a RecordingEncryption watcher that
// automatically resolves state.
type WatchConfig struct {
	Events        types.Events
	Resolver      Resolver
	ClusterConfig services.ClusterConfiguration
	Logger        *slog.Logger
	LockConfig    *backend.RunWhileLockedConfig
}

// A Watcher watches for changes to the RecordingEncryption resource and resolves the state for the calling
// auth server.
type Watcher struct {
	events        types.Events
	resolver      Resolver
	clusterConfig services.ClusterConfiguration
	logger        *slog.Logger
	lockConfig    *backend.RunWhileLockedConfig
}

// NewWatcher returns a new Watcher.
func NewWatcher(cfg WatchConfig) (*Watcher, error) {
	switch {
	case cfg.Events == nil:
		return nil, trace.BadParameter("events is required")
	case cfg.Resolver == nil:
		return nil, trace.BadParameter("recording encryption resolver is required")
	case cfg.ClusterConfig == nil:
		return nil, trace.BadParameter("cluster config backend is required")
	case cfg.LockConfig == nil:
		return nil, trace.BadParameter("lock config is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, "encryption-watcher")
	}

	return &Watcher{
		events:        cfg.Events,
		resolver:      cfg.Resolver,
		clusterConfig: cfg.ClusterConfig,
		logger:        cfg.Logger,
		lockConfig:    cfg.LockConfig,
	}, nil
}

// Watch creates a watcher responsible for responding to changes in the RecordingEncryption resource.
// This is how auth servers cooperate and ensure there are accessible wrapped keys for each unique keystore
// configuration in a cluster.
func (w *Watcher) Run(ctx context.Context) (err error) {
	jitter := func() {
		<-time.After(retryutils.SeventhJitter(time.Second * 5))
	}

	defer func() {
		w.logger.InfoContext(ctx, "stopping encryption watcher", "error", err)
	}()

	for {
		watch, err := w.events.NewWatcher(ctx, types.Watch{
			Name: "recording_encryption_watcher",
			Kinds: []types.WatchKind{
				{
					Kind: types.KindRecordingEncryption,
				},
			},
		})
		if err != nil {
			w.logger.ErrorContext(ctx, "failed to create watcher, retrying", "error", err)
			jitter()
			continue
		}

	HandleEvents:
		for {
			err := w.handleRecordingEncryptionChange(ctx)
			if err != nil {
				w.logger.ErrorContext(ctx, "failed to handle session recording config change", "error", err)
				jitter()
				continue
			}

			select {
			case ev := <-watch.Events():
				if ev.Type != types.OpPut {
					continue
				}
			case <-watch.Done():
				if err := watch.Error(); err == nil {
					return nil
				}

				w.logger.ErrorContext(ctx, "watcher failed, retrying", "error", err)
				jitter()
				break HandleEvents
			case <-ctx.Done():
				watch.Close()
				return ctx.Err()
			}
		}
	}
}

// this helper handles reacting to individual Put events on the RecordingEncryption resource and updates the
// SessionRecordingConfig with the results, if necessary
func (w *Watcher) handleRecordingEncryptionChange(ctx context.Context) error {
	return trace.Wrap(backend.RunWhileLocked(ctx, *w.lockConfig, func(ctx context.Context) error {
		recConfig, err := w.clusterConfig.GetSessionRecordingConfig(ctx)
		if err != nil {
			return trace.Wrap(err, "fetching recording config")
		}

		if !recConfig.GetEncrypted() {
			w.logger.DebugContext(ctx, "session recording encryption disabled, skip resolving keys")
			return nil
		}

		encryption, err := w.resolver.ResolveRecordingEncryption(ctx)
		if err != nil {
			w.logger.ErrorContext(ctx, "failed to resolve recording encryption state", "error", err)
			return trace.Wrap(err, "resolving recording encryption")
		}

		if recConfig.SetEncryptionKeys(GetAgeEncryptionKeys(encryption.GetSpec().ActiveKeys)) {
			_, err = w.clusterConfig.UpdateSessionRecordingConfig(ctx, recConfig)
			return trace.Wrap(err, "updating encryption keys")
		}

		return nil
	}))
}
