package services

import (
	"context"
	"crypto"

	recencpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

// RecordingEncryptionService handles key rotation requests for the keys associated with encrypted session recordings.
type RecordingEncryptionService interface {
	RotateKeySet(ctx context.Context) error
	GetRotationState(ctx context.Context) (recencpb.KeyState, error)
	CompleteRotation(ctx context.Context) error
	UploadEncryptedRecording(ctx context.Context, part chan recencpb.UploadEncryptedRecordingRequest) (chan error, error)
}

// EncryptionKeyStore provides methods for interacting with encryption keys.
type EncryptionKeyStore interface {
	NewEncryptionKeyPair(ctx context.Context, purpose cryptosuites.KeyPurpose) (*types.EncryptionKeyPair, error)
	GetDecrypter(ctx context.Context, keyPair *types.EncryptionKeyPair) (crypto.Decrypter, error)
}

// RecordingEncryptionServiceInternal extends [RecordingEncryption] with auth-specific methods.
type RecordingEncryptionServiceInternal interface {
	RecordingEncryptionService

	EvaluateRecordingEncryption(ctx context.Context, keyStore EncryptionKeyStore) (*recencpb.RecordingEncryption, error)
	CreateRecordingEncryption(ctx context.Context, encryption *recencpb.RecordingEncryption) (*recencpb.RecordingEncryption, error)
	UpdateRecordingEncryption(ctx context.Context, encryption *recencpb.RecordingEncryption) (*recencpb.RecordingEncryption, error)
	UpsertRecordingEncryption(ctx context.Context, encryption *recencpb.RecordingEncryption) (*recencpb.RecordingEncryption, error)
	GetRecordingEncryption(ctx context.Context) (*recencpb.RecordingEncryption, error)
}
