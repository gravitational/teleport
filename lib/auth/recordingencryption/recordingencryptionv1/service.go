// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package recordingencryptionv1

import (
	"bytes"
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
)

// ServiceConfig captures everything a [Service] requires to fulfill requests.
type ServiceConfig struct {
	Authorizer authz.Authorizer
	Logger     *slog.Logger
	Uploader   events.MultipartUploader
}

// NewService returns a new [Service] based on the given [ServiceConfig].
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Uploader == nil:
		return nil, trace.BadParameter("uploader is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, teleport.ComponentRecordingEncryption)
	}

	return &Service{
		logger:   cfg.Logger,
		uploader: cfg.Uploader,
		auth:     cfg.Authorizer,
	}, nil
}

// Service implements a gRPC server for interacting with encrypted recordings.
type Service struct {
	recordingencryptionv1.UnimplementedRecordingEncryptionServiceServer

	auth     authz.Authorizer
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
		return events.StreamUpload{}, trace.Wrap(err)
	}

	return events.StreamUpload{
		ID:        upload.UploadId,
		SessionID: *sessionID,
		Initiated: upload.InitiatedAt.AsTime(),
	}, nil
}

func protoAsStreamPart(part *recordingencryptionv1.Part) events.StreamPart {
	return events.StreamPart{
		Number:       part.PartNumber,
		ETag:         part.Etag,
		LastModified: time.Now(),
	}
}

// CreateUpload begins a multipart upload for an encrypted session recording.
func (s *Service) CreateUpload(ctx context.Context, req *recordingencryptionv1.CreateUploadRequest) (*recordingencryptionv1.CreateUploadResponse, error) {
	authCtx, err := s.auth.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !authz.IsLocalOrRemoteService(*authCtx) {
		return nil, trace.AccessDenied("this request can be only executed by a Teleport service")
	}

	s.logger.DebugContext(ctx, "creating encrypted session upload", "session_id", req.SessionId)
	sessionID, err := session.ParseID(req.SessionId)
	if err != nil {
		return nil, trace.Wrap(err)
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
	authCtx, err := s.auth.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !authz.IsLocalOrRemoteService(*authCtx) {
		return nil, trace.AccessDenied("this request can be only executed by a Teleport service")
	}

	s.logger.DebugContext(ctx, "uploading encrypted session part", "upload_id", req.Upload.UploadId, "session_id", req.Upload.SessionId, "part_number", req.PartNumber)
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
		Part: &recordingencryptionv1.Part{
			PartNumber: streamPart.Number,
			Etag:       streamPart.ETag,
		},
	}, nil
}

// CompleteUpload marks a given encrypted session upload as complete.
func (s *Service) CompleteUpload(ctx context.Context, req *recordingencryptionv1.CompleteUploadRequest) (*recordingencryptionv1.CompleteUploadResponse, error) {
	authCtx, err := s.auth.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !authz.IsLocalOrRemoteService(*authCtx) {
		return nil, trace.AccessDenied("this request can be only executed by a Teleport service")
	}

	s.logger.DebugContext(ctx, "completing encrypted session upload", "upload_id", req.Upload.UploadId, "session_id", req.Upload.SessionId, "parts", len(req.Parts))
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
