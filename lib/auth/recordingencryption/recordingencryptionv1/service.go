package recordingencryptionv1

import (
	"context"
	"crypto"
	"crypto/rand"
	"errors"
	"iter"
	"log/slog"

	"filippo.io/age"
	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/services"

	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
)

// Cache defines the methods required to cache RecordingEncryption resources.
type Cache interface {
	GetRecordingEncryption(ctx context.Context)
	GetRotatedKeys(ctx context.Context, publicKey []byte)
}

// EncryptionKeyStore provides methods for interacting with encryption keys.
type EncryptionKeyStore interface {
	NewEncryptionKeyPair(ctx context.Context, purpose cryptosuites.KeyPurpose) (*types.EncryptionKeyPair, error)
	GetDecrypter(ctx context.Context, keyPair *types.EncryptionKeyPair) (crypto.Decrypter, error)
}

// ServiceConfig captures everything a [Service] requires to fulfill requests.
type ServiceConfig struct {
	Logger     *slog.Logger
	Cache      Cache
	Backend    services.RecordingEncryption
	Authorizer authz.Authorizer
	Emitter    events.Emitter
	KeyStore   EncryptionKeyStore
	LockConfig backend.RunWhileLockedConfig
}

// NewService returns a new [Service] based on the given [ServiceConfig].
func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}

	if cfg.Cache == nil {
		// TODO (eriktate): replace this with an error once caching is implemented
		cfg.Cache = struct{ Cache }{}
	}

	if cfg.Backend == nil {
		return nil, trace.BadParameter("backend is required")
	}

	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required")
	}

	if cfg.Emitter == nil {
		return nil, trace.BadParameter("emitter is required")
	}

	if cfg.KeyStore == nil {
		return nil, trace.BadParameter("key store is required")
	}

	return &Service{
		logger:     cfg.Logger.With("component", teleport.ComponentRecordingEncryptionKeys),
		cache:      cfg.Cache,
		backend:    cfg.Backend,
		authorizer: cfg.Authorizer,
		emitter:    cfg.Emitter,
		keyStore:   cfg.KeyStore,
		lockConfig: cfg.LockConfig,
	}, nil
}

// Service implements the gRPC interface for interacting with RecordingEncryption resources.
type Service struct {
	recordingencryptionv1.UnimplementedRecordingEncryptionServiceServer

	logger     *slog.Logger
	cache      Cache
	authorizer authz.Authorizer
	backend    services.RecordingEncryption
	emitter    events.Emitter
	keyStore   EncryptionKeyStore
	lockConfig backend.RunWhileLockedConfig
}

// ResolveRecordingEncryption examines the current state of the RescordingEncryption resource and advances it to the
// next state on behalf of the current auth server. At a high level it will provision unfulfilled keys for itself,
// fulfill keys for keystore configurations when possible, and move its own keys through rotation states.
func (s *Service) ResolveRecordingEncryption(ctx context.Context) (*recordingencryptionv1.RecordingEncryption, error) {
	s.logger.DebugContext(ctx, "fetching recording config")
	upsert := s.backend.UpdateRecordingEncryption
	encryption, err := s.backend.GetRecordingEncryption(ctx)
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
		upsert = s.backend.CreateRecordingEncryption
	}

	s.logger.DebugContext(ctx, "recording encryption enabled, checking for active keys")
	activeKeys := encryption.GetSpec().GetKeySet().GetActiveKeys()

	// no keys present, need to generate the initial active keypair
	if len(activeKeys) == 0 {
		s.logger.DebugContext(ctx, "no active keys, generating initial keyset")
		wrappingPair, err := s.keyStore.NewEncryptionKeyPair(ctx, cryptosuites.RecordingKeyWrapping)
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
		s.logger.DebugContext(ctx, "updating session recording encryption active keys")
		encryption, err = upsert(ctx, encryption)
		return encryption, trace.Wrap(err)
	}

	s.logger.DebugContext(ctx, "searching for accessible active key")
	var activeKey *recordingencryptionv1.WrappedKey
	var decrypter crypto.Decrypter
	var unfulfilledKeys []*recordingencryptionv1.WrappedKey
	var ownUnfulfilledKey bool
	rotatingKeys := make(map[*recordingencryptionv1.WrappedKey]struct{})
	for _, key := range activeKeys {
		if key.RecordingEncryptionPair == nil {
			unfulfilledKeys = append(unfulfilledKeys, key)
		}

		dec, err := s.keyStore.GetDecrypter(ctx, key.KeyEncryptionPair)
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
			s.logger.DebugContext(ctx, "waiting for key fulfillment, nothing more to do")
			return encryption, nil
		}

		s.logger.DebugContext(ctx, "no accessible keys, generating empty key to be fulfilled")
		keypair, err := s.keyStore.NewEncryptionKeyPair(ctx, cryptosuites.RecordingKeyWrapping)
		if err != nil {
			return encryption, trace.Wrap(err, "generating keypair for new wrapped key")
		}
		activeKeys = append(activeKeys, &recordingencryptionv1.WrappedKey{
			KeyEncryptionPair: keypair,
			State:             recordingencryptionv1.KeyState_KEY_STATE_ACTIVE,
		})

		encryption.Spec.KeySet.ActiveKeys = activeKeys
		encryption, err = s.backend.UpdateRecordingEncryption(ctx, encryption)
		return encryption, trace.Wrap(err, "updating session recording config")
	}

	var shouldUpdate bool
	s.logger.DebugContext(ctx, "active key is accessible, fulfilling empty keys", "keys_waiting", len(unfulfilledKeys))
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
		s.logger.DebugContext(ctx, "marking rotated keys", "key_count", len(rotatingKeys))
	}
	for _, key := range activeKeys {
		if _, ok := rotatingKeys[key]; ok {
			key.State = recordingencryptionv1.KeyState_KEY_STATE_ROTATED
			continue
		}
	}

	if shouldUpdate {
		s.logger.DebugContext(ctx, "updating recording_encryption resource")
		encryption, err = s.backend.UpdateRecordingEncryption(ctx, encryption)
		if err != nil {
			return encryption, trace.Wrap(err, "updating session recording config")
		}
	}

	return encryption, nil
}

// RotateKeySet starts the process of rotating the active session recording encryption keypairs.
func (s *Service) RotateKeySet(ctx context.Context, req *recordingencryptionv1.RotateKeySetRequest) (*recordingencryptionv1.RotateKeySetResponse, error) {
	return nil, errors.New("unimplemented")
}

// GetRotationState returns the rotation status for a cluster. If all active keys are marked "active", no rotation is
// in progress. If at least one key is marked as "rotating", rotation is in progress. If all keys are marked "active"
// or "rotated", rotation is finished and [CompleteRotation] is ready to be called.
func (s *Service) GetRotationState(ctx context.Context, req *recordingencryptionv1.GetRotationStateRequest) (*recordingencryptionv1.GetRotationStateResponse, error) {
	return nil, errors.New("unimplemented")
}

// CompleteRotation completes key rotation for session recording encryption keys by moving all "rotated" keys into their
// own [RotatedKeys] resource indexed by the [RecordingEncryptionPair.PublicKey] shared between them.
func (s *Service) CompleteRotation(ctx context.Context, req *recordingencryptionv1.CompleteRotationRequest) (*recordingencryptionv1.CompleteRotationResponse, error) {
	return nil, errors.New("unimplemented")
}

// UploadEncryptedRecording responds to requests to upload recordings that have already been encrypted using the
// async recording mode.
func (s *Service) UploadEncryptedRecording(stream grpc.ClientStreamingServer[recordingencryptionv1.UploadEncryptedRecordingRequest, recordingencryptionv1.UploadEncryptedRecordingResponse]) error {
	return errors.New("unimplemented")
}

// WatchRecordingEncryption creates a watcher responsible for responding to changes in the RecordingEncryption
// resource. This is how auth servers cooperate and ensure there are accessible wrapped keys for each unique
// keystore configuration in a cluster.
func (s *Service) WatchRecordingEncryption(ctx context.Context, events types.Events, clusterConfig services.ClusterConfiguration) error {
	w, err := events.NewWatcher(ctx, types.Watch{
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
					err := s.handleRecordingEncryptionChange(ctx, clusterConfig)
					if err == nil {
						break
					}

					s.logger.ErrorContext(ctx, "failed to handle session recording config change", "error", err, "remaining_tries", retries-tries-1)
				}

			case <-w.Done():
				return
			}
		}
	}()

	return nil
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

// this helper handles reacting to individual Put events on the RecordingEncryption resource and updates the
// SessionRecordingConfig with the results, if necessary
func (s *Service) handleRecordingEncryptionChange(ctx context.Context, clusterConfig services.ClusterConfiguration) error {
	return trace.Wrap(backend.RunWhileLocked(ctx, s.lockConfig, func(ctx context.Context) error {
		recConfig, err := clusterConfig.GetSessionRecordingConfig(ctx)
		if err != nil {
			return trace.Wrap(err, "fetching recording config")
		}

		if !recConfig.GetEncrypted() {
			s.logger.DebugContext(ctx, "session recording encryption disabled, skip resolving keys")
			return nil
		}

		encryption, err := s.ResolveRecordingEncryption(ctx)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to resolve recording encryption state", "error", err)
			return trace.Wrap(err, "resolving recording encryption")
		}

		if recConfig.SetEncryptionKeys(GetAgeEncryptionKeys(encryption.GetSpec().GetKeySet().ActiveKeys)) {
			_, err = clusterConfig.UpdateSessionRecordingConfig(ctx, recConfig)
			return trace.Wrap(err, "updating encryption keys")
		}

		return nil
	}))
}
