package services

import (
	"context"

	recencpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
)

// RecordingEncryption handles key rotation requests for the keys associated with encrypted session recordings.
type RecordingEncryption interface {
}

// RecordingEncryptionInternal extends [RecordingEncryption] with auth-specific methods.
type RecordingEncryptionInternal interface {
	RecordingEncryption

	CreateRecordingEncryption(ctx context.Context, encryption recencpb.RecordingEncryption) (recencpb.RecordingEncryption, error)
	UpdateRecordingEncryption(ctx context.Context, encryption recencpb.RecordingEncryption) (recencpb.RecordingEncryption, error)
	UpsertRecordingEncryption(ctx context.Context, encryption recencpb.RecordingEncryption) (recencpb.RecordingEncryption, error)
}
