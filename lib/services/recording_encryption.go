package services

import (
	"context"

	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
)

// RecordingEncryption handles CRUD operations for RecordingEncryption and RotatedKeys resources.
type RecordingEncryption interface {
	// RecordingEncryption
	CreateRecordingEncryption(ctx context.Context, encryption *recordingencryptionv1.RecordingEncryption) (*recordingencryptionv1.RecordingEncryption, error)
	UpdateRecordingEncryption(ctx context.Context, encryption *recordingencryptionv1.RecordingEncryption) (*recordingencryptionv1.RecordingEncryption, error)
	GetRecordingEncryption(ctx context.Context) (*recordingencryptionv1.RecordingEncryption, error)

	// RotatedKeys
	CreateRotatedKeys(ctx context.Context, keys *recordingencryptionv1.RotatedKeys) (*recordingencryptionv1.RotatedKeys, error)
	UpdateRotatedKeys(ctx context.Context, encryption *recordingencryptionv1.RotatedKeys) (*recordingencryptionv1.RotatedKeys, error)
	GetRotatedKeys(ctx context.Context, publicKey []byte) (*recordingencryptionv1.RotatedKeys, error)
}
