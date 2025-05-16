package recordingencryptionv1

import (
	"bytes"
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"

	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
)

// ServiceConfig captures everything a [Service] requires to fulfill requests.
type ServiceConfig struct {
	Logger   *slog.Logger
	Uploader events.MultipartUploader
}

// NewService returns a new [Service] based on the given [ServiceConfig].
func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &Service{
		logger:   cfg.Logger.With("component", teleport.ComponentRecordingEncryption),
		uploader: cfg.Uploader,
	}, nil
}

// Service implements a gRPC server for interacting with encrypted recordings.
type Service struct {
	recordingencryptionv1.UnimplementedRecordingEncryptionServiceServer

	logger   *slog.Logger
	uploader events.MultipartUploader
}

func streamUploadAsProto(upload events.StreamUpload) *recordingencryptionv1.Upload {
	return &recordingencryptionv1.Upload{
		UploadId:    upload.ID,
		SessionId:   upload.SessionID.String(),
		InitiatedAt: timestamppb.New(upload.Initiated),
	}
}

func protoAsStreamUpload(upload *recordingencryptionv1.Upload) (events.StreamUpload, error) {
	sessionID, err := session.ParseID(upload.SessionId)
	if err != nil {
		return events.StreamUpload{}, trace.BadParameter("invalid session ID", err)
	}

	return events.StreamUpload{
		ID:        upload.UploadId,
		SessionID: *sessionID,
		Initiated: upload.InitiatedAt.AsTime(),
	}, nil
}

func protoAsStreamPart(part *recordingencryptionv1.UploadPartResponse) events.StreamPart {
	return events.StreamPart{
		Number:       part.PartNumber,
		ETag:         part.ETag,
		LastModified: part.LastModified.AsTime(),
	}
}

// CreateUpload begins a multipart upload for an encrypted session recording.
func (s *Service) CreateUpload(ctx context.Context, req *recordingencryptionv1.CreateUploadRequest) (*recordingencryptionv1.CreateUploadResponse, error) {
	sessionID, err := session.ParseID(req.SessionId)
	if err != nil {
		return nil, trace.BadParameter("invalid session ID", err)
	}

	upload, err := s.uploader.CreateUpload(ctx, *sessionID)
	if err != nil {
		return nil, trace.Wrap(err, "creating encrypted recording upload")
	}

	return &recordingencryptionv1.CreateUploadResponse{
		Upload: streamUploadAsProto(*upload),
	}, nil
}

// UploadPart uploads an encrypted session recording part to the given upload ID.
func (s *Service) UploadPart(ctx context.Context, req *recordingencryptionv1.UploadPartRequest) (*recordingencryptionv1.UploadPartResponse, error) {
	upload, err := protoAsStreamUpload(req.Upload)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.uploader.ReserveUploadPart(ctx, upload, req.PartNumber); err != nil {
		return nil, trace.Wrap(err)
	}

	part := bytes.NewReader(req.Part)
	streamPart, err := s.uploader.UploadPart(ctx, upload, req.PartNumber, part)
	if err != nil {
		return nil, trace.Wrap(err, "uploading encrypted recording part")
	}

	return &recordingencryptionv1.UploadPartResponse{
		PartNumber: streamPart.Number,
		ETag:       streamPart.ETag,
	}, nil
}

// CompleteUpload marks a given encrypted session upload as complete.
func (s *Service) CompleteUpload(ctx context.Context, req *recordingencryptionv1.CompleteUploadRequest) (*recordingencryptionv1.CompleteUploadResponse, error) {
	upload, err := protoAsStreamUpload(req.Upload)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	parts := make([]events.StreamPart, len(req.Parts))
	for idx, part := range req.Parts {
		parts[idx] = protoAsStreamPart(part)
	}

	if err := s.uploader.CompleteUpload(ctx, upload, parts); err != nil {
		return nil, trace.Wrap(err)
	}

	return &recordingencryptionv1.CompleteUploadResponse{}, nil
}
