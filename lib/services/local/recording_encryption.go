package local

import (
	"context"
	"crypto"
	"crypto/rand"
	"log/slog"

	"filippo.io/age"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	recencpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	recordingEncryptionPrefix = "recording_encryption"
	rotatedKeysPrefix         = recordingEncryptionPrefix + "/rotated_keys"
)

type RecordingEncryptionService struct {
	encryption  *generic.ServiceWrapper[*recencpb.RecordingEncryption]
	rotatedKeys *generic.ServiceWrapper[*recencpb.RotatedKeys]

	logger *slog.Logger
}

var _ services.RecordingEncryptionServiceInternal = (*RecordingEncryptionService)(nil)

func NewRecordingEncryptionService(b backend.Backend, logger *slog.Logger) (*RecordingEncryptionService, error) {
	const pageLimit = 100
	encryption, err := generic.NewServiceWrapper(generic.ServiceConfig[*recencpb.RecordingEncryption]{
		Backend:       b,
		PageLimit:     pageLimit,
		ResourceKind:  apitypes.KindRecordingEncryption,
		BackendPrefix: backend.NewKey(recordingEncryptionPrefix),
		MarshalFunc:   services.MarshalProtoResource[*recencpb.RecordingEncryption],
		UnmarshalFunc: services.UnmarshalProtoResource[*recencpb.RecordingEncryption],
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rotatedKeys, err := generic.NewServiceWrapper(generic.ServiceConfig[*recencpb.RotatedKeys]{
		Backend:       b,
		PageLimit:     pageLimit,
		ResourceKind:  apitypes.KindRotatedKeys,
		BackendPrefix: backend.NewKey(rotatedKeysPrefix),
		MarshalFunc:   services.MarshalProtoResource[*recencpb.RotatedKeys],
		UnmarshalFunc: services.UnmarshalProtoResource[*recencpb.RotatedKeys],
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &RecordingEncryptionService{encryption: encryption, rotatedKeys: rotatedKeys, logger: logger}, nil
}

func (s *RecordingEncryptionService) RotateKeySet(ctx context.Context) error {
	return nil
}

func (s *RecordingEncryptionService) GetRotationState(ctx context.Context) (recencpb.KeyState, error) {
	return recencpb.KeyState_KEY_STATE_UNSPECIFIED, nil
}

func (s *RecordingEncryptionService) CompleteRotation(ctx context.Context) error {
	return nil
}

func (s *RecordingEncryptionService) EvaluateRecordingEncryption(ctx context.Context, keyStore services.EncryptionKeyStore) (*recencpb.RecordingEncryption, error) {
	log := s.logger.With("component", teleport.ComponentRecordingEncryptionKeys)
	log.DebugContext(ctx, "fetching recording config")
	encryption, err := s.GetRecordingEncryption(ctx)
	if err != nil {
		if !trace.IsNotFound(err) {
			return encryption, trace.Wrap(err)
		}
		encryption = &recencpb.RecordingEncryption{
			Metadata: &headerv1.Metadata{
				Name: "recording_encryption",
			},
			Spec: &recencpb.RecordingEncryptionSpec{
				KeySet: &recencpb.KeySet{
					ActiveKeys: nil,
				},
			},
		}
	}

	log.DebugContext(ctx, "recording encryption enabled, checking for active keys")
	activeKeys := encryption.GetSpec().GetKeySet().GetActiveKeys()

	// no keys present, need to generate the initial active keypair
	if len(activeKeys) == 0 {
		log.InfoContext(ctx, "no active keys, generating initial keyset")
		wrappingPair, err := keyStore.NewEncryptionKeyPair(ctx, cryptosuites.RecordingEncryption)
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

		wrappedKey := recencpb.WrappedKey{
			KeyEncryptionPair: wrappingPair,
			RecordingEncryptionPair: &apitypes.EncryptionKeyPair{
				PrivateKeyType: apitypes.PrivateKeyType_RAW,
				PrivateKey:     encryptedIdent,
				PublicKey:      []byte(ident.Recipient().String()),
			},
			State: recencpb.KeyState_KEY_STATE_ACTIVE,
		}
		encryption.Spec.KeySet.ActiveKeys = []*recencpb.WrappedKey{&wrappedKey}
		log.DebugContext(ctx, "updating session recording encryption active keys")
		encryption, err = s.UpsertRecordingEncryption(ctx, encryption)
		return encryption, trace.Wrap(err)
	}

	log.DebugContext(ctx, "searching for accessible active key")
	var activeKey *recencpb.WrappedKey
	var decrypter crypto.Decrypter
	var unfulfilledKeys []*recencpb.WrappedKey
	var ownUnfulfilledKey bool
	rotatingKeys := make(map[*recencpb.WrappedKey]struct{})
	for _, key := range activeKeys {
		if key.RecordingEncryptionPair == nil {
			unfulfilledKeys = append(unfulfilledKeys, key)
		}

		dec, err := keyStore.GetDecrypter(ctx, key.KeyEncryptionPair)
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
		if key.State == recencpb.KeyState_KEY_STATE_ROTATING {
			rotatingKeys[key] = struct{}{}
		}
	}

	// create unfulfilled key if necessary
	if activeKey == nil || activeKey.State == recencpb.KeyState_KEY_STATE_ROTATING {
		if ownUnfulfilledKey {
			log.DebugContext(ctx, "waiting for key fulfillment, nothing more to do")
			return encryption, nil
		}

		log.InfoContext(ctx, "no accessible keys, generating empty key to be fulfilled")
		keypair, err := keyStore.NewEncryptionKeyPair(ctx, cryptosuites.RecordingEncryption)
		if err != nil {
			return encryption, trace.Wrap(err, "generating keypair for new wrapped key")
		}
		activeKeys = append(activeKeys, &recencpb.WrappedKey{
			KeyEncryptionPair: keypair,
			State:             recencpb.KeyState_KEY_STATE_ACTIVE,
		})

		encryption.Spec.KeySet.ActiveKeys = activeKeys
		encryption, err = s.UpdateRecordingEncryption(ctx, encryption)
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

			key.RecordingEncryptionPair = &apitypes.EncryptionKeyPair{
				PrivateKey: encryptedKey,
				PublicKey:  activeKey.RecordingEncryptionPair.PublicKey,
			}
			key.State = recencpb.KeyState_KEY_STATE_ACTIVE

			shouldUpdate = true
		}
	}

	log.DebugContext(ctx, "marking rotated keys", "key_count", len(rotatingKeys))
	for _, key := range activeKeys {
		if _, ok := rotatingKeys[key]; ok {
			key.State = recencpb.KeyState_KEY_STATE_ROTATED
			continue
		}
	}

	if shouldUpdate {
		log.DebugContext(ctx, "updating recording_encryption resource")
		encryption, err = s.UpdateRecordingEncryption(ctx, encryption)
		if err != nil {
			return encryption, trace.Wrap(err, "updating session recording config")
		}
	}

	return encryption, nil
}

func (s *RecordingEncryptionService) UploadEncryptedRecording(ctx context.Context, part chan recencpb.UploadEncryptedRecordingRequest) (chan error, error) {
	return nil, nil
}

func (s *RecordingEncryptionService) CreateRecordingEncryption(ctx context.Context, encryption *recencpb.RecordingEncryption) (*recencpb.RecordingEncryption, error) {
	created, err := s.encryption.CreateResource(ctx, encryption)
	return created, trace.Wrap(err)
}

func (s *RecordingEncryptionService) UpdateRecordingEncryption(ctx context.Context, encryption *recencpb.RecordingEncryption) (*recencpb.RecordingEncryption, error) {
	updated, err := s.encryption.ConditionalUpdateResource(ctx, encryption)
	return updated, trace.Wrap(err)
}

func (s *RecordingEncryptionService) UpsertRecordingEncryption(ctx context.Context, encryption *recencpb.RecordingEncryption) (*recencpb.RecordingEncryption, error) {
	upserted, err := s.encryption.UpsertResource(ctx, encryption)
	return upserted, trace.Wrap(err)
}

func (s *RecordingEncryptionService) GetRecordingEncryption(ctx context.Context) (*recencpb.RecordingEncryption, error) {
	encryption, err := s.encryption.GetResource(ctx, recordingEncryptionPrefix)
	return encryption, trace.Wrap(err)
}

func (s *RecordingEncryptionService) createRotatedKeys(ctx context.Context, rotatedKeys *recencpb.RotatedKeys) (*recencpb.RotatedKeys, error) {
	created, err := s.rotatedKeys.CreateResource(ctx, rotatedKeys)
	return created, trace.Wrap(err)
}

func (s *RecordingEncryptionService) updateRotatedKeys(ctx context.Context, rotatedKeys *recencpb.RotatedKeys) (*recencpb.RotatedKeys, error) {
	created, err := s.rotatedKeys.ConditionalUpdateResource(ctx, rotatedKeys)
	return created, trace.Wrap(err)
}

func (s *RecordingEncryptionService) upsertRotatedKeys(ctx context.Context, rotatedKeys *recencpb.RotatedKeys) (*recencpb.RotatedKeys, error) {
	created, err := s.rotatedKeys.UpsertResource(ctx, rotatedKeys)
	return created, trace.Wrap(err)
}

func (s *RecordingEncryptionService) getRotatedKeys(ctx context.Context, publicKey string) (*recencpb.RotatedKeys, error) {
	key := backend.NewKey(rotatedKeysPrefix, publicKey)
	rotatedKeys, err := s.rotatedKeys.GetResource(ctx, key.String())
	return rotatedKeys, trace.Wrap(err)
}

func (s *RecordingEncryptionService) listRotatedKeys(ctx context.Context, pageSize int, pageToken string) ([]*recencpb.RotatedKeys, string, error) {
	rotatedKeys, pageToken, err := s.rotatedKeys.ListResources(ctx, pageSize, pageToken)
	return rotatedKeys, pageToken, trace.Wrap(err)
}
