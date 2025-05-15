package local

import (
	"context"

	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	recordingEncryptionPrefix       = "recording_encryption"
	recordingEncryptionConfigPrefix = "config"
	rotatedKeysPrefix               = "rotated_keys"
)

// RecordingEncryptionService exposes backend functionality for working with the
// cluster's RecordingEncryption resource and RotateKeys.
type RecordingEncryptionService struct {
	encryption  *generic.ServiceWrapper[*recordingencryptionv1.RecordingEncryption]
	rotatedKeys *generic.ServiceWrapper[*recordingencryptionv1.RotatedKeys]
}

var _ services.RecordingEncryption = (*RecordingEncryptionService)(nil)

// NewRecordingEncryptionService creates a new RecordingEncryptionService.
func NewRecordingEncryptionService(b backend.Backend) (*RecordingEncryptionService, error) {
	const pageLimit = 100
	encryption, err := generic.NewServiceWrapper(generic.ServiceConfig[*recordingencryptionv1.RecordingEncryption]{
		Backend:       b,
		PageLimit:     pageLimit,
		ResourceKind:  types.KindRecordingEncryption,
		BackendPrefix: backend.NewKey(recordingEncryptionPrefix),
		MarshalFunc:   services.MarshalProtoResource[*recordingencryptionv1.RecordingEncryption],
		UnmarshalFunc: services.UnmarshalProtoResource[*recordingencryptionv1.RecordingEncryption],
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rotatedKeys, err := generic.NewServiceWrapper(generic.ServiceConfig[*recordingencryptionv1.RotatedKeys]{
		Backend:       b,
		PageLimit:     pageLimit,
		ResourceKind:  types.KindRotatedKeys,
		BackendPrefix: backend.NewKey(recordingEncryptionPrefix, rotatedKeysPrefix),
		MarshalFunc:   services.MarshalProtoResource[*recordingencryptionv1.RotatedKeys],
		UnmarshalFunc: services.UnmarshalProtoResource[*recordingencryptionv1.RotatedKeys],
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &RecordingEncryptionService{
		encryption:  encryption,
		rotatedKeys: rotatedKeys,
	}, nil
}

// CreateRecordingEncryption inserts a new RecordingEncryption into the backend if one
// does not already exist.
func (s *RecordingEncryptionService) CreateRecordingEncryption(ctx context.Context, encryption *recordingencryptionv1.RecordingEncryption) (*recordingencryptionv1.RecordingEncryption, error) {
	if encryption.Metadata == nil {
		encryption.Metadata = &headerv1.Metadata{}
	}
	encryption.Metadata.Name = recordingEncryptionConfigPrefix
	created, err := s.encryption.CreateResource(ctx, encryption)
	return created, trace.Wrap(err)
}

// UpdateRecordingEncryption replaces the RecordingEncryption resource with the given one.
func (s *RecordingEncryptionService) UpdateRecordingEncryption(ctx context.Context, encryption *recordingencryptionv1.RecordingEncryption) (*recordingencryptionv1.RecordingEncryption, error) {
	if encryption.Metadata == nil {
		encryption.Metadata = &headerv1.Metadata{}
	}
	encryption.Metadata.Name = recordingEncryptionConfigPrefix
	updated, err := s.encryption.ConditionalUpdateResource(ctx, encryption)
	return updated, trace.Wrap(err)
}

// GetRecordingEncryption retrieves the RecordingEncryption for the cluster.
func (s *RecordingEncryptionService) GetRecordingEncryption(ctx context.Context) (*recordingencryptionv1.RecordingEncryption, error) {
	encryption, err := s.encryption.GetResource(ctx, recordingEncryptionConfigPrefix)
	return encryption, trace.Wrap(err)
}

// CreateRotatedKeys inserts a new RotatedKeys resource into the backend.
func (s *RecordingEncryptionService) CreateRotatedKeys(ctx context.Context, rotatedKeys *recordingencryptionv1.RotatedKeys) (*recordingencryptionv1.RotatedKeys, error) {
	created, err := s.rotatedKeys.CreateResource(ctx, rotatedKeys)
	return created, trace.Wrap(err)
}

// UpdateRotatedKeys replaces a RotatedKeys resource in the backend.
func (s *RecordingEncryptionService) UpdateRotatedKeys(ctx context.Context, rotatedKeys *recordingencryptionv1.RotatedKeys) (*recordingencryptionv1.RotatedKeys, error) {
	created, err := s.rotatedKeys.ConditionalUpdateResource(ctx, rotatedKeys)
	return created, trace.Wrap(err)
}

// GetRotateKeys retrieves a RotatedKeys resource from the backend using a Bech32 encoded
// X25519 public key as the lookup.
func (s *RecordingEncryptionService) GetRotatedKeys(ctx context.Context, publicKey []byte) (*recordingencryptionv1.RotatedKeys, error) {
	rotatedKeys, err := s.rotatedKeys.GetResource(ctx, string(publicKey))
	return rotatedKeys, trace.Wrap(err)
}

type recordingEncryptionParser struct {
	baseParser
}

func newRecordingEncryptionParser() *recordingEncryptionParser {
	return &recordingEncryptionParser{
		baseParser: newBaseParser(backend.NewKey(recordingEncryptionConfigPrefix)),
	}
}

func (p *recordingEncryptionParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpPut:
		resource, err := services.UnmarshalProtoResource[*recordingencryptionv1.RecordingEncryption](
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err, "unmarshalling resource from event")
		}
		return types.Resource153ToLegacy(resource), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}
