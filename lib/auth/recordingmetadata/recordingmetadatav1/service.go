/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/player"
	"github.com/gravitational/teleport/lib/session"
)

// Service implements the RecordingMetadataServiceServer interface, providing methods to retrieve session recording
// metadata and thumbnails.
type Service struct {
	pb.UnimplementedRecordingMetadataServiceServer

	authorizer    Authorizer
	streamer      player.Streamer
	uploadHandler events.UploadHandler
	logger        *slog.Logger
}

// Authorizer is an interface that defines the method for authorizing access to session recordings.
type Authorizer interface {
	Authorize(context.Context, string) error
}

// ServiceConfig holds the configuration for the recording metadata service.
type ServiceConfig struct {
	// Authorizer is used to check if the user has permission to access the session recording.
	Authorizer Authorizer
	// Streamer is used to stream session recordings.
	Streamer player.Streamer
	// UploadHandler is used to handle uploads and downloads of session recording metadata and thumbnails.
	UploadHandler events.UploadHandler
}

var _ pb.RecordingMetadataServiceServer = (*Service)(nil)

// NewService creates a new instance of the recording metadata service.
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

// GetThumbnail retrieves the thumbnail for a session recording.
func (r *Service) GetThumbnail(ctx context.Context, req *pb.GetThumbnailRequest) (*pb.GetThumbnailResponse, error) {
	if err := r.authorizer.Authorize(ctx, req.SessionId); err != nil {
		return nil, trace.Wrap(err)
	}

	buf := &memBuffer{}
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

// GetMetadata retrieves the metadata for a session recording, streaming it back in chunks (one for metadata and one
// for each frame).
func (r *Service) GetMetadata(req *pb.GetMetadataRequest, stream grpc.ServerStreamingServer[pb.GetMetadataResponseChunk]) error {
	if err := r.authorizer.Authorize(stream.Context(), req.SessionId); err != nil {
		return trace.Wrap(err)
	}

	buf := &memBuffer{}
	err := r.uploadHandler.DownloadMetadata(stream.Context(), session.ID(req.SessionId), buf)
	if err != nil {
		return trace.Wrap(err)
	}

	metadata := &pb.SessionRecordingMetadataWithFrames{}
	err = proto.Unmarshal(buf.Bytes(), metadata)
	if err != nil {
		return trace.Wrap(err)
	}

	metadataChunk := &pb.GetMetadataResponseChunk{
		Chunk: &pb.GetMetadataResponseChunk_Metadata{
			Metadata: metadata.Metadata,
		},
	}

	if err := stream.Send(metadataChunk); err != nil {
		return trace.Wrap(err)
	}

	for _, frame := range metadata.Frames {
		frameChunk := &pb.GetMetadataResponseChunk{
			Chunk: &pb.GetMetadataResponseChunk_Frame{
				Frame: frame,
			},
		}
		if err := stream.Send(frameChunk); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}
