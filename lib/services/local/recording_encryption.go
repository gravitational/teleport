package local

import (
	"context"

	"github.com/gravitational/trace"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	recordingEncryptionPrefix = "recording_encryption"
	rotatedKeysPrefix         = recordingEncryptionPrefix + "/rotated_keys"
)

type RecordingEncryptionService struct {
	encryption  *generic.ServiceWrapper[*pb.RecordingEncryption]
	rotatedKeys *generic.ServiceWrapper[*pb.RotatedKeys]
}

var _ services.RecordingEncryption = (*RecordingEncryptionService)(nil)

func NewRecordingEncryptionService(b backend.Backend) (*RecordingEncryptionService, error) {
	const pageLimit = 100
	encryption, err := generic.NewServiceWrapper(generic.ServiceConfig[*pb.RecordingEncryption]{
		Backend:       b,
		PageLimit:     pageLimit,
		ResourceKind:  types.KindRecordingEncryption,
		BackendPrefix: backend.NewKey(recordingEncryptionPrefix),
		MarshalFunc:   services.MarshalProtoResource[*pb.RecordingEncryption],
		UnmarshalFunc: services.UnmarshalProtoResource[*pb.RecordingEncryption],
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rotatedKeys, err := generic.NewServiceWrapper(generic.ServiceConfig[*pb.RotatedKeys]{
		Backend:       b,
		PageLimit:     pageLimit,
		ResourceKind:  types.KindRotatedKeys,
		BackendPrefix: backend.NewKey(rotatedKeysPrefix),
		MarshalFunc:   services.MarshalProtoResource[*pb.RotatedKeys],
		UnmarshalFunc: services.UnmarshalProtoResource[*pb.RotatedKeys],
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &RecordingEncryptionService{
		encryption:  encryption,
		rotatedKeys: rotatedKeys,
	}, nil
}

func (s *RecordingEncryptionService) CreateRecordingEncryption(ctx context.Context, encryption *pb.RecordingEncryption) (*pb.RecordingEncryption, error) {
	created, err := s.encryption.CreateResource(ctx, encryption)
	return created, trace.Wrap(err)
}

func (s *RecordingEncryptionService) UpdateRecordingEncryption(ctx context.Context, encryption *pb.RecordingEncryption) (*pb.RecordingEncryption, error) {
	updated, err := s.encryption.ConditionalUpdateResource(ctx, encryption)
	return updated, trace.Wrap(err)
}

func (s *RecordingEncryptionService) GetRecordingEncryption(ctx context.Context) (*pb.RecordingEncryption, error) {
	encryption, err := s.encryption.GetResource(ctx, recordingEncryptionPrefix)
	return encryption, trace.Wrap(err)
}

func (s *RecordingEncryptionService) CreateRotatedKeys(ctx context.Context, rotatedKeys *pb.RotatedKeys) (*pb.RotatedKeys, error) {
	created, err := s.rotatedKeys.CreateResource(ctx, rotatedKeys)
	return created, trace.Wrap(err)
}

func (s *RecordingEncryptionService) UpdateRotatedKeys(ctx context.Context, rotatedKeys *pb.RotatedKeys) (*pb.RotatedKeys, error) {
	created, err := s.rotatedKeys.ConditionalUpdateResource(ctx, rotatedKeys)
	return created, trace.Wrap(err)
}

func (s *RecordingEncryptionService) UpsertRotatedKeys(ctx context.Context, rotatedKeys *pb.RotatedKeys) (*pb.RotatedKeys, error) {
	created, err := s.rotatedKeys.UpsertResource(ctx, rotatedKeys)
	return created, trace.Wrap(err)
}

func (s *RecordingEncryptionService) GetRotatedKeys(ctx context.Context, publicKey []byte) (*pb.RotatedKeys, error) {
	key := backend.NewKey(rotatedKeysPrefix, string(publicKey))
	rotatedKeys, err := s.rotatedKeys.GetResource(ctx, key.String())
	return rotatedKeys, trace.Wrap(err)
}

func (s *RecordingEncryptionService) ListRotatedKeys(ctx context.Context, pageSize int, pageToken string) ([]*pb.RotatedKeys, string, error) {
	rotatedKeys, pageToken, err := s.rotatedKeys.ListResources(ctx, pageSize, pageToken)
	return rotatedKeys, pageToken, trace.Wrap(err)
}

type recordingEncryptionParser struct {
	baseParser
}

func newRecordingEncryptionParser() *recordingEncryptionParser {
	return &recordingEncryptionParser{
		baseParser: newBaseParser(backend.NewKey(recordingEncryptionPrefix)),
	}
}

func (p *recordingEncryptionParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpPut:
		resource, err := services.UnmarshalProtoResource[*pb.RecordingEncryption](
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
