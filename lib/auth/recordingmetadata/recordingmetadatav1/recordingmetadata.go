/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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
	"bytes"
	"context"
	"io"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/gravitational/teleport"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	"github.com/gravitational/teleport/lib/player"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/terminal"
)

// UploadHandler uploads session recording metadata and thumbnails.
type UploadHandler interface {
	// UploadMetadata uploads session metadata and returns a URL with the uploaded
	// file in case of success. Session metadata is a file with a [recordingmetadatav1.SessionRecordingMetadata]
	// protobuf message containing info about the session (duration, events, etc), as well as
	// multiple [recordingmetadatav1.SessionRecordingThumbnail] messages (thumbnails).
	UploadMetadata(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error)
	// UploadThumbnail uploads a session thumbnail and returns a URL with uploaded
	// file in case of success. A thumbnail is [recordingmetadatav1.SessionRecordingThumbnail]
	// protobuf message which contains the thumbnail as an SVG, and some basic details about the
	// state of the terminal at the time of the thumbnail capture (terminal size, cursor position).
	UploadThumbnail(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error)
}

// RecordingMetadataService processes session recordings to generate metadata and thumbnails.
type RecordingMetadataService struct {
	logger        *slog.Logger
	streamer      player.Streamer
	uploadHandler UploadHandler
}

// RecordingMetadataServiceConfig defines the configuration for the RecordingMetadataService.
type RecordingMetadataServiceConfig struct {
	// Streamer is used to stream session events.
	Streamer player.Streamer
	// UploadHandler is used to upload session metadata and thumbnails.
	UploadHandler UploadHandler
}

const (
	// inactivityThreshold is the duration after which an inactivity event is recorded.
	inactivityThreshold = 10 * time.Second

	// maxThumbnails is the maximum number of thumbnails to store in the session metadata.
	maxThumbnails = 1000
)

// NewRecordingMetadataService creates a new instance of RecordingMetadataService with the provided configuration.
func NewRecordingMetadataService(cfg RecordingMetadataServiceConfig) (*RecordingMetadataService, error) {
	if cfg.Streamer == nil {
		return nil, trace.BadParameter("streamer is required")
	}
	if cfg.UploadHandler == nil {
		return nil, trace.BadParameter("downloadHandler is required")
	}

	return &RecordingMetadataService{
		streamer:      cfg.Streamer,
		uploadHandler: cfg.UploadHandler,
		logger:        slog.With(teleport.ComponentKey, "recording_metadata"),
	}, nil
}

// ProcessSessionRecording processes the session recording associated with the provided session ID.
// It streams session events, generates metadata, and uploads thumbnails and metadata.
func (s *RecordingMetadataService) ProcessSessionRecording(ctx context.Context, sessionID session.ID) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	processor := newSessionProcessor(sessionID)

	evts, errors := s.streamer.StreamSessionEvents(ctx, sessionID, 0)

	if err := processor.processEventStream(ctx, evts, errors); err != nil {
		return trace.Wrap(err)
	}

	metadata, thumbnails := processor.collect()

	return s.upload(ctx, sessionID, metadata, thumbnails)
}

func (s *RecordingMetadataService) upload(ctx context.Context, sessionID session.ID, metadata *pb.SessionRecordingMetadata, thumbnails []*thumbnailEntry) error {
	metadataBuf := &bytes.Buffer{}

	if _, err := protodelim.MarshalTo(metadataBuf, metadata); err != nil {
		return trace.Wrap(err)
	}

	for _, t := range thumbnails {
		if _, err := protodelim.MarshalTo(metadataBuf, thumbnailEntryToProto(t)); err != nil {
			s.logger.WarnContext(ctx, "Failed to marshal thumbnail entry",
				"session_id", sessionID, "error", err)

			continue
		}
	}

	path, err := s.uploadHandler.UploadMetadata(ctx, sessionID, metadataBuf)
	if err != nil {
		return trace.Wrap(err)
	}

	s.logger.DebugContext(ctx, "Uploaded session recording metadata", "path", path)

	thumbnail := getRandomThumbnail(thumbnails)
	if thumbnail != nil {
		b, err := proto.Marshal(thumbnailEntryToProto(thumbnail))
		if err != nil {
			return trace.Wrap(err)
		}

		path, err := s.uploadHandler.UploadThumbnail(ctx, sessionID, bytes.NewReader(b))
		if err != nil {
			return trace.Wrap(err)
		}

		s.logger.DebugContext(ctx, "Uploaded session recording thumbnail", "path", path)
	}

	return nil
}

func thumbnailEntryToProto(t *thumbnailEntry) *pb.SessionRecordingThumbnail {
	return &pb.SessionRecordingThumbnail{
		Svg:           terminal.VtStateToSvg(t.state),
		Cols:          int32(t.state.Cols),
		Rows:          int32(t.state.Rows),
		CursorX:       int32(t.state.CursorX),
		CursorY:       int32(t.state.CursorY),
		CursorVisible: t.state.CursorVisible,
		StartOffset:   durationpb.New(t.startOffset),
		EndOffset:     durationpb.New(t.endOffset),
	}
}

// getRandomThumbnail selects a random thumbnail from the middle 60% of the provided thumbnails slice.
// This tries to get a thumbnail that is more representative of the session, avoiding the very start and end.
func getRandomThumbnail(thumbnails []*thumbnailEntry) *thumbnailEntry {
	if len(thumbnails) == 0 {
		return nil
	}

	if len(thumbnails) < 5 {
		randomIndex := rand.IntN(len(thumbnails))
		return thumbnails[randomIndex]
	}

	startIndex := int(float64(len(thumbnails)) * 0.2) // start at 20%
	endIndex := int(float64(len(thumbnails)) * 0.8)   // end at 80%

	if startIndex >= endIndex {
		endIndex = startIndex + 1
	}

	rangeSize := endIndex - startIndex
	randomOffset := rand.IntN(rangeSize)
	randomIndex := startIndex + randomOffset

	return thumbnails[randomIndex]
}
