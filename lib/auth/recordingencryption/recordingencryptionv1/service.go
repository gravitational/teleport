package recordingencryptionv1

import (
	"context"
	"crypto"
	"errors"
	"log/slog"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
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
	Backend    services.RecordingEncryptionWithResolver
	Authorizer authz.Authorizer
	Emitter    events.Emitter
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

	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("emitter is required")
	}

	return &Service{
		logger:     cfg.Logger.With("component", teleport.ComponentRecordingEncryptionKeys),
		cache:      cfg.Cache,
		backend:    cfg.Backend,
		authorizer: cfg.Authorizer,
		emitter:    cfg.Emitter,
	}, nil
}

// Service implements the gRPC interface for interacting with RecordingEncryption resources.
type Service struct {
	recordingencryptionv1.UnimplementedRecordingEncryptionServiceServer

	logger     *slog.Logger
	cache      Cache
	authorizer authz.Authorizer
	backend    services.RecordingEncryptionWithResolver
	emitter    events.Emitter
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
