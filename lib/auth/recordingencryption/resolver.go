package recordingencryption

import (
	"context"
	"crypto"
	"crypto/rand"
	"iter"
	"log/slog"

	"filippo.io/age"
	"github.com/gravitational/trace"

	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/services"
)

// EncryptionKeyStore provides methods for interacting with encryption keys.
type EncryptionKeyStore interface {
	NewEncryptionKeyPair(ctx context.Context, purpose cryptosuites.KeyPurpose) (*types.EncryptionKeyPair, error)
	GetDecrypter(ctx context.Context, keyPair *types.EncryptionKeyPair) (crypto.Decrypter, error)
}

// NewRecordingEncryptionResolver wraps a services.RecordingEncryption backend with the ability to resolve keys
// against a given keystore.
func NewResolverBackend(backend services.RecordingEncryption, keyStore EncryptionKeyStore, logger *slog.Logger) (*ResolverBackend, error) {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	if backend == nil {
		return nil, trace.BadParameter("backend is required")
	}

	if keyStore == nil {
		return nil, trace.BadParameter("key store is required")
	}

	return &ResolverBackend{
		RecordingEncryption: backend,
		logger:              logger,
		keyStore:            keyStore,
	}, nil
}

// ResolverBackend resolves RecordingEncryption state using the configured backend and key store.
type ResolverBackend struct {
	services.RecordingEncryption

	logger   *slog.Logger
	keyStore EncryptionKeyStore
}

// ResolveRecordingEncryption examines the current state of the RescordingEncryption resource and advances it to the
// next state on behalf of the current auth server. At a high level it will provision unfulfilled keys for itself,
// fulfill keys for keystore configurations when possible, and move its own keys through rotation states.
func (r *ResolverBackend) ResolveRecordingEncryption(ctx context.Context) (*recordingencryptionv1.RecordingEncryption, error) {
	r.logger.DebugContext(ctx, "fetching recording config")
	upsert := r.UpdateRecordingEncryption
	encryption, err := r.GetRecordingEncryption(ctx)
	if err != nil {
		if !trace.IsNotFound(err) {
			return encryption, trace.Wrap(err)
		}
		encryption = &recordingencryptionv1.RecordingEncryption{
			Spec: &recordingencryptionv1.RecordingEncryptionSpec{
				KeySet: &recordingencryptionv1.KeySet{
					ActiveKeys: nil,
				},
			},
		}
		upsert = r.CreateRecordingEncryption
	}

	r.logger.DebugContext(ctx, "recording encryption enabled, checking for active keys")
	activeKeys := encryption.GetSpec().GetKeySet().GetActiveKeys()

	// no keys present, need to generate the initial active keypair
	if len(activeKeys) == 0 {
		r.logger.DebugContext(ctx, "no active keys, generating initial keyset")
		wrappingPair, err := r.keyStore.NewEncryptionKeyPair(ctx, cryptosuites.RecordingKeyWrapping)
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
		encryption.Spec.KeySet.ActiveKeys = []*recordingencryptionv1.WrappedKey{&wrappedKey}
		r.logger.DebugContext(ctx, "updating session recording encryption active keys")
		encryption, err = upsert(ctx, encryption)
		return encryption, trace.Wrap(err)
	}

	r.logger.DebugContext(ctx, "searching for accessible active key")
	var activeKey *recordingencryptionv1.WrappedKey
	var decrypter crypto.Decrypter
	var unfulfilledKeys []*recordingencryptionv1.WrappedKey
	var ownUnfulfilledKey bool
	rotatingKeys := make(map[*recordingencryptionv1.WrappedKey]struct{})
	for _, key := range activeKeys {
		if key.RecordingEncryptionPair == nil {
			unfulfilledKeys = append(unfulfilledKeys, key)
		}

		dec, err := r.keyStore.GetDecrypter(ctx, key.KeyEncryptionPair)
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
			r.logger.DebugContext(ctx, "waiting for key fulfillment, nothing more to do")
			return encryption, nil
		}

		r.logger.DebugContext(ctx, "no accessible keys, generating empty key to be fulfilled")
		keypair, err := r.keyStore.NewEncryptionKeyPair(ctx, cryptosuites.RecordingKeyWrapping)
		if err != nil {
			return encryption, trace.Wrap(err, "generating keypair for new wrapped key")
		}
		activeKeys = append(activeKeys, &recordingencryptionv1.WrappedKey{
			KeyEncryptionPair: keypair,
			State:             recordingencryptionv1.KeyState_KEY_STATE_ACTIVE,
		})

		encryption.Spec.KeySet.ActiveKeys = activeKeys
		encryption, err = r.UpdateRecordingEncryption(ctx, encryption)
		return encryption, trace.Wrap(err, "updating session recording config")
	}

	var shouldUpdate bool
	r.logger.DebugContext(ctx, "active key is accessible, fulfilling empty keys", "keys_waiting", len(unfulfilledKeys))
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
		r.logger.DebugContext(ctx, "marking rotated keys", "key_count", len(rotatingKeys))
	}
	for _, key := range activeKeys {
		if _, ok := rotatingKeys[key]; ok {
			key.State = recordingencryptionv1.KeyState_KEY_STATE_ROTATED
			continue
		}
	}

	if shouldUpdate {
		r.logger.DebugContext(ctx, "updating recording_encryption resource")
		encryption, err = r.UpdateRecordingEncryption(ctx, encryption)
		if err != nil {
			return encryption, trace.Wrap(err, "updating session recording config")
		}
	}

	return encryption, nil
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

// RecordingEncryptionWatchConfig captures required dependencies for building a RecordingEncyprtion watcher that
// automatically resolves state.
type RecordingEncryptionWatchConfig struct {
	Events              types.Events
	RecordingEncryption services.RecordingEncryptionWithResolver
	ClusterConfig       services.ClusterConfiguration
	Logger              *slog.Logger
	LockConfig          backend.RunWhileLockedConfig
}

// Watch creates a watcher responsible for responding to changes in the RecordingEncryption
// resource. This is how auth servers cooperate and ensure there are accessible wrapped keys for each unique
// keystore configuration in a cluster.
func Watch(ctx context.Context, cfg RecordingEncryptionWatchConfig) error {
	switch {
	case cfg.Events == nil:
		return trace.BadParameter("events is required")
	case cfg.RecordingEncryption == nil:
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

		encryption, err := cfg.RecordingEncryption.ResolveRecordingEncryption(ctx)
		if err != nil {
			cfg.Logger.ErrorContext(ctx, "failed to resolve recording encryption state", "error", err)
			return trace.Wrap(err, "resolving recording encryption")
		}

		if recConfig.SetEncryptionKeys(GetAgeEncryptionKeys(encryption.GetSpec().GetKeySet().ActiveKeys)) {
			_, err = cfg.ClusterConfig.UpdateSessionRecordingConfig(ctx, recConfig)
			return trace.Wrap(err, "updating encryption keys")
		}

		return nil
	}))
}
