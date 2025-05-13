package services

import (
	"context"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
)

// RecordingEncryption handles CRUD operations for RecordingEncryption and RotatedKeys resources.
type RecordingEncryption interface {
	// RecordingEncryption
	CreateRecordingEncryption(ctx context.Context, encryption *pb.RecordingEncryption) (*pb.RecordingEncryption, error)
	UpdateRecordingEncryption(ctx context.Context, encryption *pb.RecordingEncryption) (*pb.RecordingEncryption, error)
	GetRecordingEncryption(ctx context.Context) (*pb.RecordingEncryption, error)

	// RotatedKeys
	CreateRotatedKeys(ctx context.Context, keys *pb.RotatedKeys) (*pb.RotatedKeys, error)
	UpdateRotatedKeys(ctx context.Context, encryption *pb.RotatedKeys) (*pb.RotatedKeys, error)
	GetRotatedKeys(ctx context.Context, publicKey []byte) (*pb.RotatedKeys, error)
}
