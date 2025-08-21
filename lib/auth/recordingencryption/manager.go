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
	"encoding/hex"
	"iter"
	"log/slog"
	"slices"
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
	"github.com/gravitational/teleport/lib/utils"
)

// KeyStore provides methods for interacting with encryption keys.
type KeyStore interface {
	NewEncryptionKeyPair(ctx context.Context, purpose cryptosuites.KeyPurpose) (*types.EncryptionKeyPair, error)
	GetDecrypter(ctx context.Context, keyPair *types.EncryptionKeyPair) (crypto.Decrypter, error)
	FindDecryptersByLabels(ctx context.Context, labels ...*types.KeyLabel) ([]crypto.Decrypter, error)
}

// A Cache fetches a cached [*recordingencryptionv1.RecordingEncryption].
type Cache interface {
	GetRecordingEncryption(context.Context) (*recordingencryptionv1.RecordingEncryption, error)
}

// ManagerConfig captures all of the dependencies required to instantiate a Manager.
type ManagerConfig struct {
	Backend                   services.RecordingEncryption
	ClusterConfig             services.ClusterConfigurationInternal
	KeyStore                  KeyStore
	Cache                     Cache
	Logger                    *slog.Logger
	LockConfig                backend.RunWhileLockedConfig
	ManualKeyManagementConfig *types.ManualKeyManagementConfig
}

// NewManager returns a new Manager using the given [ManagerConfig].
func NewManager(ctx context.Context, cfg ManagerConfig) (*Manager, error) {
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

		ctx:             ctx,
		cache:           cfg.Cache,
		keyStore:        cfg.KeyStore,
		lockConfig:      cfg.LockConfig,
		logger:          cfg.Logger,
		manualKeyConfig: cfg.ManualKeyManagementConfig,
	}, nil
}

// A Manager wraps a services.RecordingEncryption and KeyStore in order to provide more complex operations
// than the CRUD methods exposed by services.RecordingEncryption. It primarily handles resolving RecordingEncryption
// state and searching for accessible decryption keys.
type Manager struct {
	services.RecordingEncryption
	services.ClusterConfigurationInternal

	ctx             context.Context
	cache           Cache
	keyStore        KeyStore
	keyCache        utils.SyncMap[string, crypto.Decrypter]
	lockConfig      backend.RunWhileLockedConfig
	logger          *slog.Logger
	manualKeyConfig *types.ManualKeyManagementConfig
}

// CreateSessionRecordingConfig creates a new session recording configuration. If encryption is enabled then an
// accessible encryption key pair will be confirmed. Either creating one if none exists, doing nothing if one is
// accessible, or returning an error if none are accessible.
func (m *Manager) CreateSessionRecordingConfig(ctx context.Context, cfg types.SessionRecordingConfig) (sessionRecordingConfig types.SessionRecordingConfig, err error) {
	err = backend.RunWhileLocked(ctx, m.lockConfig, func(ctx context.Context) error {
		encryptionCfg := cfg.GetEncryptionConfig()
		if encryptionCfg != nil && encryptionCfg.Enabled {
			encryption, err := m.ensureRecordingEncryptionKey(ctx, *encryptionCfg)
			if err != nil {
				return err
			}

			_ = cfg.SetEncryptionKeys(getAgeEncryptionKeys(encryption.GetSpec().ActiveKeyPairs))
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
		encryptionCfg := cfg.GetEncryptionConfig()
		if encryptionCfg != nil && encryptionCfg.Enabled {
			encryption, err := m.ensureRecordingEncryptionKey(ctx, *encryptionCfg)
			if err != nil {
				return err
			}

			_ = cfg.SetEncryptionKeys(getAgeEncryptionKeys(encryption.GetSpec().ActiveKeyPairs))
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
		encryptionCfg := cfg.GetEncryptionConfig()
		if encryptionCfg != nil && encryptionCfg.Enabled {
			encryption, err := m.ensureRecordingEncryptionKey(ctx, *encryptionCfg)
			if err != nil {
				return err
			}

			_ = cfg.SetEncryptionKeys(getAgeEncryptionKeys(encryption.GetSpec().ActiveKeyPairs))
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

type fingerprintedDecrypter struct {
	fingerprint string
	decrypter   crypto.Decrypter
}

func (m *Manager) ensureManualEncryptionKeys(manualKeyCfg types.ManualKeyManagementConfig) (*recordingencryptionv1.RecordingEncryption, error) {
	m.manualKeyConfig = &manualKeyCfg
	activeLabels := manualKeyCfg.ActiveKeys
	rotatedLabels := manualKeyCfg.RotatedKeys

	// using the Manager's context here because we cache the resulting keys and want their lifetimes
	// to be at least as long as the Manager
	activeDecrypters, err := m.keyStore.FindDecryptersByLabels(m.ctx, activeLabels...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rotatedDecrypters, err := m.keyStore.FindDecryptersByLabels(m.ctx, rotatedLabels...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var fingerprinted []fingerprintedDecrypter
	for _, decrypter := range slices.Concat(rotatedDecrypters, activeDecrypters) {
		fp, err := Fingerprint(decrypter.Public())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		fingerprinted = append(fingerprinted, fingerprintedDecrypter{
			fingerprint: fp,
			decrypter:   decrypter,
		})
	}

	m.keyCache.Write(func(cache map[string]crypto.Decrypter) {
		for _, dec := range fingerprinted {
			cache[dec.fingerprint] = dec.decrypter
		}
	})

	var encryptionKeys []*recordingencryptionv1.KeyPair
	for _, decrypter := range activeDecrypters {
		pubKey, err := keys.MarshalPublicKey(decrypter.Public())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		encryptionKeys = append(encryptionKeys, &recordingencryptionv1.KeyPair{
			KeyPair: &types.EncryptionKeyPair{
				PublicKey: pubKey,
			},
		})
	}
	return &recordingencryptionv1.RecordingEncryption{
		Spec: &recordingencryptionv1.RecordingEncryptionSpec{
			ActiveKeyPairs: encryptionKeys,
		},
	}, nil
}

// ensureRecordingEncryptionKey returns the configured RecordingEncryption resource if it exists with an
// accessible key. If no keys exist, a new key pair will be provisioned. An error is returned if keys exist
// but none are accessible.
func (m *Manager) ensureRecordingEncryptionKey(ctx context.Context, encryptionCfg types.SessionRecordingEncryptionConfig) (*recordingencryptionv1.RecordingEncryption, error) {
	if encryptionCfg.ManualKeyManagement != nil && encryptionCfg.ManualKeyManagement.Enabled {
		return m.ensureManualEncryptionKeys(*encryptionCfg.ManualKeyManagement)
	}

	m.manualKeyConfig = nil
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

	activePairs := encryption.GetSpec().ActiveKeyPairs
	if len(activePairs) > 0 {
		for _, pair := range activePairs {
			// fetch the decrypter to ensure we have access to it
			if _, err := m.keyStore.GetDecrypter(ctx, pair.KeyPair); err != nil {
				fp, _ := fingerprintPEM(pair.KeyPair.PublicKey)
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

	wrappedKey := recordingencryptionv1.KeyPair{
		KeyPair: encryptionPair,
	}
	encryption.Spec.ActiveKeyPairs = []*recordingencryptionv1.KeyPair{&wrappedKey}
	encryption, err = persistFn(ctx, encryption)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fp, _ := fingerprintPEM(encryptionPair.PublicKey)
	m.logger.InfoContext(ctx, "no active keys, generated initial recording encryption pair", "public_fingerprint", fp)
	return encryption, nil
}

func (m *Manager) unwrapKeyUsingCache(in UnwrapInput) ([]byte, error) {
	if decrypter, ok := m.keyCache.Load(in.Fingerprint); ok {
		fileKey, err := decrypter.Decrypt(in.Rand, in.WrappedKey, in.Opts)
		return fileKey, trace.Wrap(err)
	}

	return nil, nil
}

// UnwrapKey searches for the private key compatible with the provided public key fingerprint and uses it to unwrap
// a wrapped file key.
func (m *Manager) UnwrapKey(ctx context.Context, in UnwrapInput) ([]byte, error) {
	fileKey, err := m.unwrapKeyUsingCache(in)
	if fileKey != nil && err == nil {
		return fileKey, nil
	}

	// a cache miss or unwrap failure for manually managed keys needs to attempt a refresh and try again
	if m.manualKeyConfig != nil && m.manualKeyConfig.Enabled {
		if _, err := m.ensureManualEncryptionKeys(*m.manualKeyConfig); err != nil {
			return nil, trace.Wrap(err)
		}

		fileKey, err = m.unwrapKeyUsingCache(in)
		return fileKey, trace.Wrap(err)
	}

	// a cache miss in for teleport managed keys just needs to fall back to the keystore
	if err != nil {
		m.logger.WarnContext(ctx, "failed to unwrap file key using cached decrypter, refetching from keystore")
	}

	encryption, err := m.cache.GetRecordingEncryption(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO (eriktate): search rotated keys as well once rotation is implemented
	activePairs := encryption.GetSpec().ActiveKeyPairs
	for _, key := range activePairs {
		if key.GetKeyPair() == nil {
			continue
		}

		activeFP, err := fingerprintPEM(key.KeyPair.PublicKey)
		if err != nil {
			m.logger.ErrorContext(ctx, "failed to fingerprint active public key", "error", err)
			continue
		}

		if activeFP != in.Fingerprint {
			continue
		}

		decrypter, err := m.keyStore.GetDecrypter(ctx, key.KeyPair)
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

	// getNextManualSync returns a timed channel meant to trigger encryption key syncing when keys are
	// manually managed
	getNextManualSync := func() <-chan time.Time {
		return time.After(retryutils.SeventhJitter(time.Minute * 5))
	}

	defer func() {
		m.logger.InfoContext(ctx, "stopping encryption watcher", "error", err)
	}()

	// on initial startup we should try to immediately resolve recording encryption
	if err := m.resolveRecordingEncryption(ctx, shouldRetryAfterJitterFn); err != nil {
		m.logger.ErrorContext(ctx, "initial attempt to resolve recording encryption failed", "error", err)
	}

	nextSync := getNextManualSync()
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
				if ev.Type != types.OpPut || ev.Resource.GetKind() != types.KindRecordingEncryption {
					continue
				}
				if err := m.resolveRecordingEncryption(ctx, shouldRetryAfterJitterFn); err != nil {
					m.logger.ErrorContext(ctx, "failure handling recording encryption event", "kind", ev.Resource.GetKind(), "error", err)
					continue
				}
				// reset interval sync since we just resolved recording encryption state
				nextSync = getNextManualSync()
			case <-nextSync:
				nextSync = getNextManualSync()
				if m.manualKeyConfig == nil || !m.manualKeyConfig.Enabled {
					// we only need to sync on an interval when keys are manually managed
					continue
				}

				if err := m.resolveRecordingEncryption(ctx, shouldRetryAfterJitterFn); err != nil {
					m.logger.ErrorContext(ctx, "failed interval sync of recording encryption keys", "error", err)
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

func (m *Manager) resolveRecordingEncryption(ctx context.Context, shouldRetryFn func() bool) error {
	const retries = 3
	for retry := range retries {
		err := backend.RunWhileLocked(ctx, m.lockConfig, func(ctx context.Context) error {
			sessionRecordingConfig, err := m.GetSessionRecordingConfig(ctx)
			if err != nil {
				m.logger.ErrorContext(ctx, "failed to retrieve session_recording_config, retrying", "error", err)
				return trace.Wrap(err)
			}

			encryptionCfg := sessionRecordingConfig.GetEncryptionConfig()
			if encryptionCfg == nil || !encryptionCfg.Enabled {
				return nil
			}

			encryption, err := m.ensureRecordingEncryptionKey(ctx, *encryptionCfg)
			if err != nil {
				m.logger.ErrorContext(ctx, "failed to resolve recording encryption keys, retrying", "retry", retry, "retries_left", retries-retry, "error", err)
				return trace.Wrap(err)
			}

			if sessionRecordingConfig.SetEncryptionKeys(getAgeEncryptionKeys(encryption.GetSpec().ActiveKeyPairs)) {
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
func getAgeEncryptionKeys(keys []*recordingencryptionv1.KeyPair) iter.Seq[*types.AgeEncryptionKey] {
	return func(yield func(*types.AgeEncryptionKey) bool) {
		for _, key := range keys {
			if key.KeyPair == nil {
				continue
			}

			if !yield(&types.AgeEncryptionKey{
				PublicKey: key.KeyPair.PublicKey,
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
	return hex.EncodeToString(fp[:]), nil
}

// fingerprints a public RSA key encoded as PEM-wrapped PKIX.
func fingerprintPEM(pubKeyPEM []byte) (string, error) {
	pubKey, err := keys.ParsePublicKey(pubKeyPEM)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return Fingerprint(pubKey)
}
