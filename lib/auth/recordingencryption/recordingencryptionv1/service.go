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

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
)

type Cache interface {
	GetRecordingEncryption(ctx context.Context)
	GetRotatedKeys(ctx context.Context, publicKey []byte)
}

// EncryptionKeyStore provides methods for interacting with encryption keys.
type EncryptionKeyStore interface {
	NewEncryptionKeyPair(ctx context.Context, purpose cryptosuites.KeyPurpose) (*types.EncryptionKeyPair, error)
	GetDecrypter(ctx context.Context, keyPair *types.EncryptionKeyPair) (crypto.Decrypter, error)
}

type ServiceConfig struct {
	Logger     *slog.Logger
	Cache      Cache
	Backend    services.RecordingEncryption
	Authorizer authz.Authorizer
	Emitter    events.Emitter
	KeyStore   EncryptionKeyStore
	LockConfig backend.RunWhileLockedConfig
}

func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}

	if cfg.Cache == nil {
		return nil, trace.BadParameter("cache is required")
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
		logger:     cfg.Logger,
		cache:      cfg.Cache,
		backend:    cfg.Backend,
		authorizer: cfg.Authorizer,
		emitter:    cfg.Emitter,
		keyStore:   cfg.KeyStore,
		lockConfig: cfg.LockConfig,
	}, nil
}

type Service struct {
	pb.UnimplementedRecordingEncryptionServiceServer

	logger     *slog.Logger
	cache      Cache
	authorizer authz.Authorizer
	backend    services.RecordingEncryption
	emitter    events.Emitter
	keyStore   EncryptionKeyStore
	lockConfig backend.RunWhileLockedConfig
}

func (s *Service) ResolveRecordingEncryption(ctx context.Context) (*pb.RecordingEncryption, error) {
	log := s.logger.With("component", teleport.ComponentRecordingEncryptionKeys)
	log.DebugContext(ctx, "fetching recording config")
	upsert := s.backend.UpdateRecordingEncryption
	encryption, err := s.backend.GetRecordingEncryption(ctx)
	if err != nil {
		if !trace.IsNotFound(err) {
			return encryption, trace.Wrap(err)
		}
		encryption = &pb.RecordingEncryption{
			Metadata: &headerv1.Metadata{
				Name: "recording_encryption",
			},
			Spec: &pb.RecordingEncryptionSpec{
				KeySet: &pb.KeySet{
					ActiveKeys: nil,
				},
			},
		}
		upsert = s.backend.CreateRecordingEncryption
	}

	log.DebugContext(ctx, "recording encryption enabled, checking for active keys")
	activeKeys := encryption.GetSpec().GetKeySet().GetActiveKeys()

	// no keys present, need to generate the initial active keypair
	if len(activeKeys) == 0 {
		log.InfoContext(ctx, "no active keys, generating initial keyset")
		wrappingPair, err := s.keyStore.NewEncryptionKeyPair(ctx, cryptosuites.RecordingEncryption)
		if err != nil {
			return encryption, trace.Wrap(err, "generating wrapping key")
		}

		ident, err := age.GenerateX25519Identity()
		if err != nil {
			return encryption, trace.Wrap(err, "generating age encryption key")
		}

		encryptedIdent, err := wrappingPair.EncryptionKey().EncryptOAEP([]byte(ident.String()))
		if err != nil {
			return encryption, trace.Wrap(err, "wrapping encryption key")
		}

		wrappedKey := pb.WrappedKey{
			KeyEncryptionPair: wrappingPair,
			RecordingEncryptionPair: &types.EncryptionKeyPair{
				PrivateKeyType: types.PrivateKeyType_RAW,
				PrivateKey:     encryptedIdent,
				PublicKey:      []byte(ident.Recipient().String()),
			},
			State: pb.KeyState_KEY_STATE_ACTIVE,
		}
		encryption.Spec.KeySet.ActiveKeys = []*pb.WrappedKey{&wrappedKey}
		log.DebugContext(ctx, "updating session recording encryption active keys")
		encryption, err = upsert(ctx, encryption)
		return encryption, trace.Wrap(err)
	}

	log.DebugContext(ctx, "searching for accessible active key")
	var activeKey *pb.WrappedKey
	var decrypter crypto.Decrypter
	var unfulfilledKeys []*pb.WrappedKey
	var ownUnfulfilledKey bool
	rotatingKeys := make(map[*pb.WrappedKey]struct{})
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
		if key.State == pb.KeyState_KEY_STATE_ROTATING {
			rotatingKeys[key] = struct{}{}
		}
	}

	// create unfulfilled key if necessary
	if activeKey == nil || activeKey.State == pb.KeyState_KEY_STATE_ROTATING {
		if ownUnfulfilledKey {
			log.DebugContext(ctx, "waiting for key fulfillment, nothing more to do")
			return encryption, nil
		}

		log.InfoContext(ctx, "no accessible keys, generating empty key to be fulfilled")
		keypair, err := s.keyStore.NewEncryptionKeyPair(ctx, cryptosuites.RecordingEncryption)
		if err != nil {
			return encryption, trace.Wrap(err, "generating keypair for new wrapped key")
		}
		activeKeys = append(activeKeys, &pb.WrappedKey{
			KeyEncryptionPair: keypair,
			State:             pb.KeyState_KEY_STATE_ACTIVE,
		})

		encryption.Spec.KeySet.ActiveKeys = activeKeys
		encryption, err = s.backend.UpdateRecordingEncryption(ctx, encryption)
		return encryption, trace.Wrap(err, "updating session recording config")
	}

	var shouldUpdate bool
	log.DebugContext(ctx, "active key is accessible, fulfilling empty keys", "keys_waiting", len(unfulfilledKeys))
	if len(unfulfilledKeys) > 0 {
		decryptionKey, err := decrypter.Decrypt(rand.Reader, activeKey.RecordingEncryptionPair.PrivateKey, nil)
		if err != nil {
			return encryption, trace.Wrap(err, "decrypting known key")
		}

		for _, key := range unfulfilledKeys {
			encryptedKey, err := key.KeyEncryptionPair.EncryptionKey().EncryptOAEP(decryptionKey)
			if err != nil {
				return encryption, trace.Wrap(err, "reencrypting decryption key")
			}

			key.RecordingEncryptionPair = &types.EncryptionKeyPair{
				PrivateKey: encryptedKey,
				PublicKey:  activeKey.RecordingEncryptionPair.PublicKey,
			}
			key.State = pb.KeyState_KEY_STATE_ACTIVE

			shouldUpdate = true
		}
	}

	if len(rotatingKeys) > 0 {
		log.DebugContext(ctx, "marking rotated keys", "key_count", len(rotatingKeys))
	}
	for _, key := range activeKeys {
		if _, ok := rotatingKeys[key]; ok {
			key.State = pb.KeyState_KEY_STATE_ROTATED
			continue
		}
	}

	if shouldUpdate {
		log.DebugContext(ctx, "updating recording_encryption resource")
		encryption, err = s.backend.UpdateRecordingEncryption(ctx, encryption)
		if err != nil {
			return encryption, trace.Wrap(err, "updating session recording config")
		}
	}

	return encryption, nil
}

func (s *Service) RotateKeySet(ctx context.Context, req *pb.RotateKeySetRequest) (*pb.RotateKeySetResponse, error) {
	return nil, errors.New("unimplemented")
}

func (s *Service) GetRotationState(ctx context.Context, req *pb.GetRotationStateRequest) (*pb.GetRotationStateResponse, error) {
	return nil, errors.New("unimplemented")
}

func (s *Service) CompleteRotation(ctx context.Context, req *pb.CompleteRotationRequest) (*pb.CompleteRotationResponse, error) {
	return nil, errors.New("unimplemented")
}

func (s *Service) UploadEncryptedRecording(stream grpc.ClientStreamingServer[pb.UploadEncryptedRecordingRequest, pb.UploadEncryptedRecordingResponse]) error {
	return errors.New("unimplemented")
}

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
				for range retries {
					err := s.handleRecordingEncryptionChange(ctx, clusterConfig)
					if err == nil {
						break
					}

					s.logger.ErrorContext(ctx, "failed to handle session recording config change", "error", err)
				}

			case <-w.Done():
				return
			}
		}
	}()

	return nil
}

func EncryptedKeyIter(keys []*pb.WrappedKey) iter.Seq[*types.EncryptionKey] {
	return func(yield func(*types.EncryptionKey) bool) {
		for _, key := range keys {
			if !yield(&types.EncryptionKey{
				PublicKey: key.RecordingEncryptionPair.PublicKey,
				Hash:      key.RecordingEncryptionPair.Hash,
			}) {
				return
			}
		}
	}
}

func (s *Service) handleRecordingEncryptionChange(ctx context.Context, clusterConfig services.ClusterConfiguration) error {
	// lockCfg := backend.RunWhileLockedConfig{
	// 	LockConfiguration: backend.LockConfiguration{
	// 		Backend:            asrv.bk,
	// 		LockNameComponents: []string{"session_recording_config_watcher"},
	// 		TTL:                30 * time.Second,
	// 	},
	// 	RefreshLockInterval: 20 * time.Second,
	// }

	return trace.Wrap(backend.RunWhileLocked(ctx, s.lockConfig, func(ctx context.Context) error {
		recConfig, err := clusterConfig.GetSessionRecordingConfig(ctx)
		if err != nil {
			return trace.Wrap(err, "fetching recording config")
		}

		if !recConfig.GetEncrypted() {
			s.logger.InfoContext(ctx, "recording encryption disabled, bailing")
			return nil
		}

		encryption, err := s.ResolveRecordingEncryption(ctx)
		if err != nil {
			s.logger.ErrorContext(ctx, "failed to evaluate recording_encryption", "error", err)
			return trace.Wrap(err)
		}

		if recConfig.SetEncryptionKeys(EncryptedKeyIter(encryption.GetSpec().GetKeySet().ActiveKeys)) {
			_, err = clusterConfig.UpdateSessionRecordingConfig(ctx, recConfig)
			return trace.Wrap(err)
		}

		return nil
	}))
}
