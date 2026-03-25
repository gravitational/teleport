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
	Backend                       services.RecordingEncryption
	ClusterConfig                 services.ClusterConfigurationInternal
	KeyStore                      KeyStore
	Cache                         Cache
	Logger                        *slog.Logger
	LockConfig                    backend.RunWhileLockedConfig
	ManualKeyManagementConfig     *types.ManualKeyManagementConfig
	InitialSessionRecordingConfig types.SessionRecordingConfig
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

	mgr := &Manager{
		RecordingEncryption:          cfg.Backend,
		ClusterConfigurationInternal: cfg.ClusterConfig,

		ctx:             ctx,
		cache:           cfg.Cache,
		keyStore:        cfg.KeyStore,
		lockConfig:      cfg.LockConfig,
		logger:          cfg.Logger,
		manualKeyConfig: cfg.ManualKeyManagementConfig,
	}

	if cfg.InitialSessionRecordingConfig != nil {
		manualKeyCfg := getManualKeyManagement(cfg.InitialSessionRecordingConfig.GetEncryptionConfig())
		if manualKeyCfg != nil {
			if _, err := mgr.ensureManualEncryptionKeys(ctx, *manualKeyCfg); err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}

	return mgr, nil
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
func (m *Manager) CreateSessionRecordingConfig(ctx context.Context, cfg types.SessionRecordingConfig) (types.SessionRecordingConfig, error) {
	sessionRecordingConfig, err := m.modifySessionRecordingConfig(ctx, cfg, m.ClusterConfigurationInternal.CreateSessionRecordingConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sessionRecordingConfig, trace.Wrap(err)
}

// UpdateSessionRecordingConfig updates an existing session recording configuration.  If encryption is enabled
// then an accessible encryption key pair will be confirmed. Either creating one if none exists, doing nothing
// if one is accessible, or returning an error if none are accessible.
func (m *Manager) UpdateSessionRecordingConfig(ctx context.Context, cfg types.SessionRecordingConfig) (types.SessionRecordingConfig, error) {
	sessionRecordingConfig, err := m.modifySessionRecordingConfig(ctx, cfg, m.ClusterConfigurationInternal.UpdateSessionRecordingConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sessionRecordingConfig, trace.Wrap(err)
}

// UpsertSessionRecordingConfig creates a new session recording configuration or overwrites an existing one. If
// encryption is enabled then an accessible encryption key pair will be confirmed. Either creating one if none
// exists, doing nothing if one is accessible, or returning an error if none are accessible.
func (m *Manager) UpsertSessionRecordingConfig(ctx context.Context, cfg types.SessionRecordingConfig) (types.SessionRecordingConfig, error) {
	sessionRecordingConfig, err := m.modifySessionRecordingConfig(ctx, cfg, m.ClusterConfigurationInternal.UpsertSessionRecordingConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sessionRecordingConfig, trace.Wrap(err)
}

// SetCache overwrites the configured Cache implementation. It should only be called if the `Manager` is not in use.
func (m *Manager) SetCache(cache Cache) {
	m.cache = cache
}

// fingerprintedDecrypter wraps a crypto.Decrypter with its public key fingerprint.
type fingerprintedDecrypter struct {
	fingerprint string
	decrypter   crypto.Decrypter
}

// ensureManualEncryptionKeys searches for accessible encryption keys in the configured key store using the key labels
// configured in the [ManualKeyManagementConfig]
func (m *Manager) ensureManualEncryptionKeys(ctx context.Context, manualKeyCfg types.ManualKeyManagementConfig) (*recordingencryptionv1.RecordingEncryption, error) {
	m.manualKeyConfig = &manualKeyCfg
	activeLabels := manualKeyCfg.ActiveKeys
	rotatedLabels := manualKeyCfg.RotatedKeys

	// using the Manager's context here because we cache the resulting keys and want their lifetimes
	// to be at least as long as the Manager
	activeDecrypters, err := m.keyStore.FindDecryptersByLabels(ctx, activeLabels...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(activeDecrypters) == 0 {
		return nil, trace.BadParameter("No accessible key found for manual_key_management.active_keys")
	}

	rotatedDecrypters, err := m.keyStore.FindDecryptersByLabels(ctx, rotatedLabels...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(rotatedDecrypters) == 0 && len(rotatedLabels) > 0 {
		// Failure to retrieve rotated keys should only prevent replay of historical recordings
		// related to those keys,  which isn't catastrophic in the way that missing active keys are.
		// We should log the failure but otherwise press on.
		m.logger.WarnContext(ctx, "No accessible rotated keys were found which may prevent replaying historical session recordings. Please ensure the Teleport Auth Service has access to the keystore and keys defined in manual_key_management.rotated_keys")
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
		pubKey, err := x509.MarshalPKIXPublicKey(decrypter.Public())
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

// ensureActiveKeyPair checks that there is at least one accessible key in the list of active pairs given.
// If there are no active keys found, a new one will be provisioned and returned with the original set.
// An active but inaccessible key results in an error.
func (m *Manager) ensureActiveKeyPair(ctx context.Context, activePairs []*recordingencryptionv1.KeyPair) (newActivePairs []*recordingencryptionv1.KeyPair, keyGenerated bool, err error) {
	var foundActiveKey bool
	if len(activePairs) > 0 {
		for _, pair := range activePairs {
			if pair.State != recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_ACTIVE {
				continue
			}

			foundActiveKey = true
			// fetch the decrypter to ensure we have access to it
			if _, err := m.keyStore.GetDecrypter(ctx, pair.KeyPair); err != nil {
				fp, _ := fingerprintPEM(pair.KeyPair.PublicKey)
				m.logger.DebugContext(ctx, "key not accessible", "fingerprint", fp)
				continue
			}
			return activePairs, false, nil
		}

		// during a key rotation there may be no active keys which would mean reaching
		// this point isn't an error
		if foundActiveKey {
			return nil, false, trace.AccessDenied("active key not accessible")
		}
	}

	// no active keys present, need to generate one
	encryptionPair, err := m.keyStore.NewEncryptionKeyPair(ctx, cryptosuites.RecordingKeyWrapping)
	if err != nil {
		return nil, false, trace.Wrap(err, "generating wrapping key")
	}

	fp, _ := fingerprintPEM(encryptionPair.PublicKey)
	m.logger.InfoContext(ctx, "no active encryption keys, generated new pair", "public_fingerprint", fp)

	return append(activePairs, &recordingencryptionv1.KeyPair{
		KeyPair: encryptionPair,
		State:   recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_ACTIVE,
	}), true, nil
}

// resolveRecordingEncryption returns the configured [RecordingEncryption] resource if it exists with an
// accessible key. If no active keys can be found and manual key management is disabled, a new key pair
// will be provisioned. An error is returned if an active key is found but is not accessible. It is also
// possible for the returned [RecordingEncryption] to be nil if the [SessionRecordingConfig] disables
// encryption altogether.
func (m *Manager) resolveRecordingEncryption(ctx context.Context, sessionRecordingCfg types.SessionRecordingConfig, encryption *recordingencryptionv1.RecordingEncryption) (*recordingencryptionv1.RecordingEncryption, error) {
	// If encryption is disabled then we can skip resolving the RecordingEncryption.
	if !sessionRecordingCfg.GetEncrypted() {
		return encryption, nil
	}
	encryptionCfg := sessionRecordingCfg.GetEncryptionConfig()
	manualKeyCfg := getManualKeyManagement(encryptionCfg)
	if manualKeyCfg != nil {
		encryption, err := m.ensureManualEncryptionKeys(ctx, *manualKeyCfg)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return encryption, nil
	}

	m.manualKeyConfig = nil
	persistFn := m.RecordingEncryption.UpdateRecordingEncryption
	forcePersist := encryption != nil
	if encryption == nil {
		var err error
		encryption, err = m.RecordingEncryption.GetRecordingEncryption(ctx)
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			encryption = &recordingencryptionv1.RecordingEncryption{
				Spec: &recordingencryptionv1.RecordingEncryptionSpec{},
			}
			persistFn = m.RecordingEncryption.CreateRecordingEncryption
		}
	}

	activePairs, keyGenerated, err := m.ensureActiveKeyPair(ctx, encryption.GetSpec().GetActiveKeyPairs())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !forcePersist && !keyGenerated {
		return encryption, nil
	}

	encryption.Spec.ActiveKeyPairs = activePairs
	encryption, err = persistFn(ctx, encryption)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return encryption, nil
}

func (m *Manager) unwrapKeyUsingCache(in UnwrapInput) ([]byte, error) {
	if decrypter, ok := m.keyCache.Load(in.Fingerprint); ok {
		fileKey, err := decrypter.Decrypt(in.Rand, in.WrappedKey, in.Opts)
		return fileKey, trace.Wrap(err)
	}

	return nil, nil
}

func (m *Manager) findDecrypter(ctx context.Context, fingerprint string) (crypto.Decrypter, error) {
	encryption, err := m.cache.GetRecordingEncryption(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	activePairs := encryption.GetSpec().GetActiveKeyPairs()
	for _, key := range activePairs {
		if key.GetKeyPair() == nil {
			continue
		}

		activeFP, err := fingerprintPEM(key.KeyPair.PublicKey)
		if err != nil {
			m.logger.ErrorContext(ctx, "failed to fingerprint active public key", "error", err)
			continue
		}

		if activeFP != fingerprint {
			continue
		}

		decrypter, err := m.keyStore.GetDecrypter(ctx, key.KeyPair)
		if err != nil {
			continue
		}

		return decrypter, nil
	}

	// fallback to rotated keys if decrypter isn't found in active keys
	rotatedKey, err := m.GetRotatedKey(ctx, fingerprint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	decrypter, err := m.keyStore.GetDecrypter(ctx, rotatedKey.GetSpec().GetEncryptionKeyPair())
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("no accessible decrypter found for fingerprint %q", fingerprint)
		}

		return nil, trace.Wrap(err)
	}

	return decrypter, nil
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
		if _, err := m.ensureManualEncryptionKeys(ctx, *m.manualKeyConfig); err != nil {
			return nil, trace.Wrap(err)
		}

		fileKey, err = m.unwrapKeyUsingCache(in)
		return fileKey, trace.Wrap(err)
	}

	// a cache miss in for teleport managed keys just needs to fall back to the keystore
	if err != nil {
		m.logger.WarnContext(ctx, "failed to unwrap file key using cached decrypter, refetching from keystore")
	}

	decrypter, err := m.findDecrypter(ctx, in.Fingerprint)
	if err != nil {
		return nil, trace.Wrap(err)

	}

	fileKey, err = decrypter.Decrypt(in.Rand, in.WrappedKey, in.Opts)
	return fileKey, trace.Wrap(err)
}

// RotateKey marks the currently active key pair as 'rotating' and adds a new pair to the active set.
func (m *Manager) RotateKey(ctx context.Context) error {
	sessionRecordingConfig, err := m.GetSessionRecordingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := shouldManageKeys(sessionRecordingConfig); err != nil {
		return trace.Wrap(err)
	}

	encryption, err := m.GetRecordingEncryption(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	activePairs := encryption.GetSpec().GetActiveKeyPairs()
	var activePair *recordingencryptionv1.KeyPair
	for _, pair := range activePairs {
		switch pair.GetState() {
		case recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_ROTATING:
			return trace.AlreadyExists("Recording encryption key rotation already in progress, complete or rollback to start a new one")
		case recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_INACCESSIBLE:
			return trace.BadParameter("Failed recording encryption key rotation in progress, at least one Teleport Auth Service does not have access to the new key pair. Please rollback the rotation, ensure that all Auth Services have access to provision and use keys within the configured keystore, and then try again.")
		case recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_ACTIVE:
			activePair = pair
		}
	}

	if activePair == nil {
		return trace.NotFound("no active key present to rotate")
	}

	activePair.State = recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_ROTATING
	if _, err := m.modifyRecordingEncryption(ctx, encryption); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// CompleteRotation creates a new [RotatedKey] resource for rotating key pair and removes them from the active set.
func (m *Manager) CompleteRotation(ctx context.Context) error {
	sessionRecordingConfig, err := m.GetSessionRecordingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := shouldManageKeys(sessionRecordingConfig); err != nil {
		return trace.Wrap(err)
	}

	encryption, err := m.GetRecordingEncryption(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	activePairs := encryption.GetSpec().GetActiveKeyPairs()
	var remainingPairs []*recordingencryptionv1.KeyPair
	var rotatedPairs []*recordingencryptionv1.KeyPair
	for _, pair := range activePairs {
		switch pair.GetState() {
		case recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_ROTATING:
			rotatedPairs = append(rotatedPairs, pair)
		case recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_ACTIVE:
			remainingPairs = append(remainingPairs, pair)
		case recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_INACCESSIBLE:
			return trace.BadParameter("Failed recording encryption key rotation in progress, at least one Teleport Auth Service does not have access to the new key pair. Please rollback the rotation, ensure that all Auth Services have access to provision and use keys within the configured keystore, and then try again.")
		}
	}

	for _, pair := range rotatedPairs {
		if _, err := m.CreateRotatedKey(ctx, pair.GetKeyPair()); err != nil {
			return trace.Wrap(err)
		}
	}

	encryption.Spec.ActiveKeyPairs = remainingPairs
	if _, err := m.modifyRecordingEncryption(ctx, encryption); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// RollbackRotation reverts an in progress rotation by setting 'rotating' keys back to 'active' and removing
// all other keys from the active set.
func (m *Manager) RollbackRotation(ctx context.Context) error {
	sessionRecordingConfig, err := m.GetSessionRecordingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := shouldManageKeys(sessionRecordingConfig); err != nil {
		return trace.Wrap(err)
	}

	encryption, err := m.GetRecordingEncryption(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	activePairs := encryption.GetSpec().GetActiveKeyPairs()
	var rollbackPairs []*recordingencryptionv1.KeyPair
	for _, pair := range activePairs {
		if pair.GetState() == recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_ROTATING {
			pair.State = recordingencryptionv1.KeyPairState_KEY_PAIR_STATE_ACTIVE
			rollbackPairs = append(rollbackPairs, pair)
		}
	}

	if len(rollbackPairs) == 0 {
		return trace.BadParameter("skipping rollback that would remove all encryption keys")
	}

	encryption.Spec.ActiveKeyPairs = rollbackPairs
	if _, err := m.modifyRecordingEncryption(ctx, encryption); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetRotationState returns the state of each active key identified by a fingerprint.
func (m *Manager) GetRotationState(ctx context.Context) ([]*recordingencryptionv1.FingerprintWithState, error) {
	sessionRecordingConfig, err := m.GetSessionRecordingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := shouldManageKeys(sessionRecordingConfig); err != nil {
		return nil, trace.Wrap(err)
	}

	encryption, err := m.GetRecordingEncryption(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	activePairs := encryption.GetSpec().GetActiveKeyPairs()
	if len(activePairs) == 0 {
		return nil, nil
	}

	states := make([]*recordingencryptionv1.FingerprintWithState, 0, len(activePairs))
	for _, pair := range activePairs {
		fingerprint, err := fingerprintPEM(pair.GetKeyPair().PublicKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		states = append(states, &recordingencryptionv1.FingerprintWithState{
			Fingerprint: fingerprint,
			State:       pair.GetState(),
		})
	}

	return states, nil
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
	if err := m.resolveRecordingEncryptionState(ctx, shouldRetryAfterJitterFn); err != nil {
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
				if err := m.resolveRecordingEncryptionState(ctx, shouldRetryAfterJitterFn); err != nil {
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

				if err := m.resolveRecordingEncryptionState(ctx, shouldRetryAfterJitterFn); err != nil {
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

// modifySessionRecordingConfig modifies the [SessionRecordingConfig] while maintaining the correct
// [RecordingEncryption] state. Any changes to active encryption keys are also applied to the [SessionRecordingConfig]
// before applying the change.
func (m *Manager) modifySessionRecordingConfig(ctx context.Context, cfg types.SessionRecordingConfig, persistFn func(context.Context, types.SessionRecordingConfig) (types.SessionRecordingConfig, error)) (types.SessionRecordingConfig, error) {
	var result types.SessionRecordingConfig
	err := backend.RunWhileLocked(ctx, m.lockConfig, func(ctx context.Context) error {
		encryption, err := m.resolveRecordingEncryption(ctx, cfg, nil)
		if err != nil {
			return trace.Wrap(err)
		}

		_ = cfg.SetEncryptionKeys(getAgeEncryptionKeys(encryption.GetSpec().GetActiveKeyPairs()))
		result, err = persistFn(ctx, cfg)
		return trace.Wrap(err)
	})

	return result, trace.Wrap(err)
}

// modifyRecordingEncryption modifies the [RecordingEncryption] if the fetched [SessionRecordingConfig] allows it.
// Any changes to active encryption keys are also applied to the [SessionRecordingConfig] before releasing the lock.
func (m *Manager) modifyRecordingEncryption(ctx context.Context, encryption *recordingencryptionv1.RecordingEncryption) (*recordingencryptionv1.RecordingEncryption, error) {
	var result *recordingencryptionv1.RecordingEncryption
	err := backend.RunWhileLocked(ctx, m.lockConfig, func(ctx context.Context) error {
		sessionRecordingCfg, err := m.GetSessionRecordingConfig(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		result, err = m.resolveRecordingEncryption(ctx, sessionRecordingCfg, encryption)
		if err != nil {
			return trace.Wrap(err)
		}

		if sessionRecordingCfg.SetEncryptionKeys(getAgeEncryptionKeys(result.GetSpec().GetActiveKeyPairs())) {
			if _, err = m.ClusterConfigurationInternal.UpdateSessionRecordingConfig(ctx, sessionRecordingCfg); err != nil {
				return trace.Wrap(err)
			}
		}

		return nil
	})

	return result, trace.Wrap(err)
}

// resolveRecordingEncryptionState fetches the latest [SessionRecordingConfig] and [RecordingEncryption] resources
// from the backend and applies any updates the calling auth instance might require. This is meant to be run on an
// interval or in response to watch events. The primary updates expected are key state changes for inaccessible
// encryption keys and refreshing the manual key management cache. If the current state cannot be resolved after a
// number of retries, a [trace.LimitExceededError] is returned.
func (m *Manager) resolveRecordingEncryptionState(ctx context.Context, shouldRetryFn func() bool) error {
	const retries = 3
	for retry := range retries {
		if _, err := m.modifyRecordingEncryption(ctx, nil); err != nil {
			m.logger.ErrorContext(ctx, "failed to resolve recording encryption keys, retrying", "retry", retry, "retries_left", retries-retry, "error", err)
			if shouldRetryFn() {
				continue
			}

			return trace.Wrap(err)
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

func getManualKeyManagement(encryptionCfg *types.SessionRecordingEncryptionConfig) *types.ManualKeyManagementConfig {
	if encryptionCfg == nil {
		return nil
	}

	if encryptionCfg.ManualKeyManagement == nil || !encryptionCfg.ManualKeyManagement.Enabled {
		return nil
	}

	return encryptionCfg.ManualKeyManagement
}

func shouldManageKeys(sessionRecordingConfig types.SessionRecordingConfig) error {
	encryptionCfg := sessionRecordingConfig.GetEncryptionConfig()
	if encryptionCfg == nil || !encryptionCfg.Enabled {
		return trace.NotFound("session recording encryption is not enabled")
	}

	if manual := getManualKeyManagement(encryptionCfg); manual != nil && manual.Enabled {
		return trace.NotFound("session recording encryption is using manual key management")
	}

	return nil
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
	pubKey, err := x509.ParsePKIXPublicKey(pubKeyPEM)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return Fingerprint(pubKey)
}
