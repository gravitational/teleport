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
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"iter"
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/services"
)

// KeyStore provides methods for interacting with encryption keys.
type KeyStore interface {
	NewEncryptionKeyPair(ctx context.Context, purpose cryptosuites.KeyPurpose) (*types.EncryptionKeyPair, error)
	GetDecrypter(ctx context.Context, keyPair *types.EncryptionKeyPair) (crypto.Decrypter, error)
}

// A Cache fetches a cached [*recordingencryptionv1.RecordingEncryption].
type Cache interface {
	GetRecordingEncryption(context.Context) (*recordingencryptionv1.RecordingEncryption, error)
}

// ManagerConfig captures all of the dependencies required to instantiate a Manager.
type ManagerConfig struct {
	Backend       services.RecordingEncryption
	ClusterConfig services.ClusterConfigurationInternal
	KeyStore      KeyStore
	Cache         Cache
	Logger        *slog.Logger
	LockConfig    backend.RunWhileLockedConfig
}

// NewManager returns a new Manager using the given [ManagerConfig].
func NewManager(cfg ManagerConfig) (*Manager, error) {
	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend is required")
	case cfg.ClusterConfig == nil:
		return nil, trace.BadParameter("cluster config is required")
	case cfg.KeyStore == nil:
		return nil, trace.BadParameter("key store is required")
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, "recording-encryption-manager")
	}

	return &Manager{
		RecordingEncryption:          cfg.Backend,
		ClusterConfigurationInternal: cfg.ClusterConfig,

		cache:      cfg.Cache,
		keyStore:   cfg.KeyStore,
		lockConfig: cfg.LockConfig,
		logger:     cfg.Logger,
	}, nil
}

// A Manager wraps a services.RecordingEncryption and KeyStore in order to provide more complex operations
// than the CRUD methods exposed by services.RecordingEncryption. It primarily handles resolving RecordingEncryption
// state and searching for accessible decryption keys.
type Manager struct {
	services.RecordingEncryption
	services.ClusterConfigurationInternal

	cache      Cache
	logger     *slog.Logger
	lockConfig backend.RunWhileLockedConfig
	keyStore   KeyStore
}

// CreateSessionRecordingConfig creates a new session recording configuration. If encryption is enabled then an
// accessible encryption key pair will be confirmed. Either creating one if none exists, doing nothing if one is
// accessible, or returning an error if none are accessible.
func (m *Manager) CreateSessionRecordingConfig(ctx context.Context, cfg types.SessionRecordingConfig) (sessionRecordingConfig types.SessionRecordingConfig, err error) {
	err = backend.RunWhileLocked(ctx, m.lockConfig, func(ctx context.Context) error {
		if cfg.GetEncrypted() {
			encryption, err := m.ensureRecordingEncryptionKey(ctx)
			if err != nil {
				return err
			}

			_ = cfg.SetEncryptionKeys(getAgeEncryptionKeys(encryption.GetSpec().ActiveKeys))
		}

		sessionRecordingConfig, err = m.ClusterConfigurationInternal.CreateSessionRecordingConfig(ctx, cfg)
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	})

	return sessionRecordingConfig, trace.Wrap(err)
}

// UpdateSessionRecordingConfig updates an existing session recording configuration.  If encryption is enabled
// then an accessible encryption key pair will be confirmed. Either creating one if none exists, doing nothing
// if one is accessible, or returning an error if none are accessible.
func (m *Manager) UpdateSessionRecordingConfig(ctx context.Context, cfg types.SessionRecordingConfig) (sessionRecordingConfig types.SessionRecordingConfig, err error) {
	err = backend.RunWhileLocked(ctx, m.lockConfig, func(ctx context.Context) error {
		if cfg.GetEncrypted() {
			encryption, err := m.ensureRecordingEncryptionKey(ctx)
			if err != nil {
				return err
			}

			_ = cfg.SetEncryptionKeys(getAgeEncryptionKeys(encryption.GetSpec().ActiveKeys))
		}

		sessionRecordingConfig, err = m.ClusterConfigurationInternal.UpdateSessionRecordingConfig(ctx, cfg)
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	})

	return sessionRecordingConfig, trace.Wrap(err)
}

// UpsertSessionRecordingConfig creates a new session recording configuration or overwrites an existing one. If
// encryption is enabled then an accessible encryption key pair will be confirmed. Either creating one if none
// exists, doing nothing if one is accessible, or returning an error if none are accessible.
func (m *Manager) UpsertSessionRecordingConfig(ctx context.Context, cfg types.SessionRecordingConfig) (sessionRecordingConfig types.SessionRecordingConfig, err error) {
	err = backend.RunWhileLocked(ctx, m.lockConfig, func(ctx context.Context) error {
		if cfg.GetEncrypted() {
			encryption, err := m.ensureRecordingEncryptionKey(ctx)
			if err != nil {
				return err
			}

			_ = cfg.SetEncryptionKeys(getAgeEncryptionKeys(encryption.GetSpec().ActiveKeys))
		}

		sessionRecordingConfig, err = m.ClusterConfigurationInternal.UpsertSessionRecordingConfig(ctx, cfg)
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	})

	return sessionRecordingConfig, trace.Wrap(err)
}

// SetCache overwrites the configured Cache implementation. It should only be called if the `Manager` is not in use.
func (m *Manager) SetCache(cache Cache) {
	m.cache = cache
}

// ensureRecordingEncryptionKey returns the configured RecordingEncryption resource if it exists with an
// accessible key. If no keys exist, a new key pair will be provisioned. An error is returned if keys exist
// but none are accessible.
func (m *Manager) ensureRecordingEncryptionKey(ctx context.Context) (*recordingencryptionv1.RecordingEncryption, error) {
	persistFn := m.RecordingEncryption.UpdateRecordingEncryption
	encryption, err := m.RecordingEncryption.GetRecordingEncryption(ctx)
	if err != nil {
		if !trace.IsNotFound(err) {
			return encryption, trace.Wrap(err)
		}
		encryption = &recordingencryptionv1.RecordingEncryption{
			Spec: &recordingencryptionv1.RecordingEncryptionSpec{},
		}
		persistFn = m.RecordingEncryption.CreateRecordingEncryption
	}

	activeKeys := encryption.GetSpec().ActiveKeys
	if len(activeKeys) > 0 {
		for _, key := range activeKeys {
			// fetch the decrypter to ensure we have access to it
			if _, err := m.keyStore.GetDecrypter(ctx, key.RecordingEncryptionPair); err != nil {
				fp, _ := fingerprintPEM(key.RecordingEncryptionPair.PublicKey)
				m.logger.DebugContext(ctx, "key not accessible", "fingerprint", fp)
				continue
			}
			return encryption, nil
		}

		return nil, trace.AccessDenied("active key not accessible: %v", err)
	}

	// no keys present, need to generate the initial active keypair
	encryptionPair, err := m.keyStore.NewEncryptionKeyPair(ctx, cryptosuites.RecordingKeyWrapping)
	if err != nil {
		return nil, trace.Wrap(err, "generating wrapping key")
	}

	wrappedKey := recordingencryptionv1.WrappedKey{
		RecordingEncryptionPair: encryptionPair,
	}
	encryption.Spec.ActiveKeys = []*recordingencryptionv1.WrappedKey{&wrappedKey}
	encryption, err = persistFn(ctx, encryption)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fp, _ := fingerprintPEM(encryptionPair.PublicKey)
	m.logger.InfoContext(ctx, "no active keys, generated initial recording encryption pair", "public_fingerprint", fp)
	return encryption, nil
}

// UnwrapKey searches for the private key compatible with the provided public key fingerprint and uses it to unwrap
// a wrapped file key.
func (m *Manager) UnwrapKey(ctx context.Context, in UnwrapInput) ([]byte, error) {
	encryption, err := m.cache.GetRecordingEncryption(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO (eriktate): search rotated keys as well once rotation is implemented
	activeKeys := encryption.GetSpec().ActiveKeys
	for _, key := range activeKeys {
		if key.GetRecordingEncryptionPair() == nil {
			continue
		}

		activeFP, err := fingerprintPEM(key.RecordingEncryptionPair.PublicKey)
		if err != nil {
			m.logger.ErrorContext(ctx, "failed to fingerprint active public key", "error", err)
			continue
		}

		if activeFP != in.Fingerprint {
			continue
		}

		decrypter, err := m.keyStore.GetDecrypter(ctx, key.RecordingEncryptionPair)
		if err != nil {
			continue
		}

		fileKey, err := decrypter.Decrypt(in.Rand, in.WrappedKey, in.Opts)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return fileKey, nil
	}

	return nil, trace.NotFound("no accessible decrypter found")
}

// Watch for changes in the recording_encryption resource and respond by ensuring access to keys.
func (m *Manager) Watch(ctx context.Context, events types.Events) (err error) {
	// shouldRetryAfterJitterFn waits at most 5 seconds and returns a bool specifying whether or not
	// execution should continue
	shouldRetryAfterJitterFn := func() bool {
		select {
		case <-time.After(retryutils.SeventhJitter(time.Second * 5)):
			return true
		case <-ctx.Done():
			return false
		}
	}

	defer func() {
		m.logger.InfoContext(ctx, "stopping encryption watcher", "error", err)
	}()

	for {
		watch, err := events.NewWatcher(ctx, types.Watch{
			Name: "recording_encryption_watcher",
			Kinds: []types.WatchKind{
				{
					Kind: types.KindRecordingEncryption,
				},
			},
		})
		if err != nil {
			m.logger.ErrorContext(ctx, "failed to create watcher, retrying", "error", err)
			if !shouldRetryAfterJitterFn() {
				return nil
			}
			continue
		}
		defer watch.Close()

	HandleEvents:
		for {
			select {
			case ev := <-watch.Events():
				if err := m.handleEvent(ctx, ev, shouldRetryAfterJitterFn); err != nil {
					m.logger.ErrorContext(ctx, "failure handling recording encryption event", "kind", ev.Resource.GetKind(), "error", err)
				}
			case <-watch.Done():
				if err := watch.Error(); err == nil {
					return nil
				}

				m.logger.ErrorContext(ctx, "watcher failed, retrying", "error", err)
				if !shouldRetryAfterJitterFn() {
					return nil
				}
				break HandleEvents
			case <-ctx.Done():
				return nil
			}

		}
	}
}

func (m *Manager) handleEvent(ctx context.Context, ev types.Event, shouldRetryFn func() bool) error {
	if ev.Type != types.OpPut {
		return nil
	}

	if ev.Resource.GetKind() != types.KindRecordingEncryption {
		return nil
	}

	const retries = 3
	for retry := range retries {
		err := backend.RunWhileLocked(ctx, m.lockConfig, func(ctx context.Context) error {
			sessionRecordingConfig, err := m.GetSessionRecordingConfig(ctx)
			if err != nil {
				m.logger.ErrorContext(ctx, "failed to retrieve session_recording_config, retrying", "error", err)
				return trace.Wrap(err)
			}

			if !sessionRecordingConfig.GetEncrypted() {
				return nil
			}

			encryption, err := m.ensureRecordingEncryptionKey(ctx)
			if err != nil {
				m.logger.ErrorContext(ctx, "failed to resolve recording encryption keys, retrying", "retry", retry, "retries_left", retries-retry, "error", err)
				return trace.Wrap(err)
			}

			if sessionRecordingConfig.SetEncryptionKeys(getAgeEncryptionKeys(encryption.GetSpec().ActiveKeys)) {
				if _, err := m.ClusterConfigurationInternal.UpdateSessionRecordingConfig(ctx, sessionRecordingConfig); err != nil {
					return trace.Wrap(err)
				}
			}

			return nil
		})
		if err != nil && shouldRetryFn() {
			continue
		}

		return nil
	}

	return trace.LimitExceeded("resolving recording encryption exceeded max retries")
}

// getAgeEncryptionKeys returns an iterator of AgeEncryptionKeys from a list of WrappedKeys. This is for use in
// populating the EncryptionKeys field of SessionRecordingConfigStatus.
func getAgeEncryptionKeys(keys []*recordingencryptionv1.WrappedKey) iter.Seq[*types.AgeEncryptionKey] {
	return func(yield func(*types.AgeEncryptionKey) bool) {
		for _, key := range keys {
			if key.RecordingEncryptionPair == nil {
				continue
			}

			if !yield(&types.AgeEncryptionKey{
				PublicKey: key.RecordingEncryptionPair.PublicKey,
			}) {
				return
			}
		}
	}
}

// Fingerprint a public key for use in logging and as a cache key.
func Fingerprint(pubKey crypto.PublicKey) (string, error) {
	derPub, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return "", trace.Wrap(err)
	}

	fp := sha256.Sum256(derPub)
	return base64.StdEncoding.EncodeToString(fp[:]), nil
}

// fingerprints a public RSA key encoded as PEM-wrapped PKIX.
func fingerprintPEM(pubKeyPEM []byte) (string, error) {
	pubKey, err := keys.ParsePublicKey(pubKeyPEM)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return Fingerprint(pubKey)
}
