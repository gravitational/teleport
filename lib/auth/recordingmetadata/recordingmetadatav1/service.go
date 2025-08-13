/**
 * Copyright (C) 2024 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package recordingmetadatav1

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/player"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils/membuffer"
)

type Service struct {
	pb.UnimplementedRecordingMetadataServiceServer

	authorizer    Authorizer
	streamer      player.Streamer
	uploadHandler events.UploadHandler
	logger        *slog.Logger
}

type Authorizer interface {
	Authorize(context.Context, string) error
}

type ServiceConfig struct {
	Authorizer    Authorizer
	Streamer      player.Streamer
	UploadHandler events.UploadHandler
}

var _ pb.RecordingMetadataServiceServer = (*Service)(nil)

func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required")
	}
	if cfg.UploadHandler == nil {
		return nil, trace.BadParameter("upload handler is required")
	}

	return &Service{
		authorizer:    cfg.Authorizer,
		streamer:      cfg.Streamer,
		uploadHandler: cfg.UploadHandler,
		logger:        slog.With(teleport.ComponentKey, "recording_metadata"),
	}, nil
}

func (r *Service) GetThumbnail(ctx context.Context, req *pb.GetThumbnailRequest) (*pb.GetThumbnailResponse, error) {
	if err := r.authorizer.Authorize(ctx, req.SessionId); err != nil {
		return nil, trace.Wrap(err)
	}

	buf := &membuffer.MemBuffer{}
	err := r.uploadHandler.DownloadThumbnail(ctx, session.ID(req.SessionId), buf)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	thumbnail := &pb.SessionRecordingThumbnail{}
	err = proto.Unmarshal(buf.Bytes(), thumbnail)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &pb.GetThumbnailResponse{Thumbnail: thumbnail}, nil
}

func (r *Service) GetMetadata(ctx context.Context, req *pb.GetMetadataRequest) (*pb.GetMetadataResponse, error) {
	if err := r.authorizer.Authorize(ctx, req.SessionId); err != nil {
		return nil, trace.Wrap(err)
	}
	buf := &membuffer.MemBuffer{}
	err := r.uploadHandler.DownloadMetadata(ctx, session.ID(req.SessionId), buf)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	metadata := &pb.SessionRecordingMetadata{}
	err = proto.Unmarshal(buf.Bytes(), metadata)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &pb.GetMetadataResponse{Metadata: metadata}, nil
}
