/**
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
	"bufio"
	"bytes"
	"context"
	"log/slog"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/hinshun/vt10x"
	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/player"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/terminal"
)

// RecordingMetadataService processes session recordings to generate metadata and thumbnails.
type RecordingMetadataService struct {
	logger        *slog.Logger
	streamer      player.Streamer
	uploadHandler events.UploadHandler
}

// RecordingMetadataServiceConfig defines the configuration for the RecordingMetadataService.
type RecordingMetadataServiceConfig struct {
	// Streamer is used to stream session events.
	Streamer player.Streamer
	// UploadHandler is used to upload session metadata and thumbnails.
	UploadHandler events.UploadHandler
}

const (
	// inactivityThreshold is the duration after which an inactivity event is recorded.
	inactivityThreshold = 5 * time.Second

	// maxThumbnails is the maximum number of thumbnails to store in the session metadata.
	maxThumbnails = 1000
)

// NewRecordingMetadataService creates a new instance of RecordingMetadataService with the provided configuration.
func NewRecordingMetadataService(cfg RecordingMetadataServiceConfig) *RecordingMetadataService {
	return &RecordingMetadataService{
		streamer:      cfg.Streamer,
		uploadHandler: cfg.UploadHandler,
		logger:        slog.With(teleport.ComponentKey, "recording_metadata"),
	}
}

// ProcessSessionRecording processes the session recording associated with the provided session ID.
// It streams session events, generates metadata, and uploads thumbnails and metadata.
func (s *RecordingMetadataService) ProcessSessionRecording(ctx context.Context, sessionID session.ID) error {
	evts, errors := s.streamer.StreamSessionEvents(ctx, sessionID, 0)

	var startTime time.Time
	var lastEvent apievents.AuditEvent
	var lastActivityTime time.Time

	thumbnailInterval := 1 * time.Second
	activeUsers := make(map[string]time.Duration)

	vt := vt10x.New()

	frames := make([]*pb.SessionRecordingThumbnail, 0)
	metadata := &pb.SessionRecordingMetadata{
		Events: make([]*pb.SessionRecordingEvent, 0),
	}

	addInactivityEvent := func(start, end time.Time) {
		if end.IsZero() {
			return
		}

		inactivityStart := durationpb.New(start.Sub(startTime))
		inactivityEnd := durationpb.New(end.Sub(startTime))

		metadata.Events = append(metadata.Events, &pb.SessionRecordingEvent{
			StartOffset: inactivityStart,
			EndOffset:   inactivityEnd,
			Event: &pb.SessionRecordingEvent_Inactivity{
				Inactivity: &pb.SessionRecordingInactivityEvent{},
			},
		})
	}

	sampler := newThumbnailBucketSampler(maxThumbnails, thumbnailInterval)

	recordThumbnail := func(start time.Time) {
		state := vt.DumpState()

		sampler.add(&state, start)
	}

loop:
	for {
		select {
		case evt := <-evts:
			if evt == nil {
				break loop
			}

			lastEvent = evt

			switch e := evt.(type) {
			case *apievents.DatabaseSessionStart, *apievents.WindowsDesktopSessionStart:
				// Unsupported session recording types
				return nil

			case *apievents.Resize:
				parts := strings.Split(e.TerminalSize, ":")

				if len(parts) == 2 {
					cols, rows, err := parseTerminalSize(e.TerminalSize)
					if err != nil {
						return trace.Wrap(err, "failed to parse terminal size %q for session %v", e.TerminalSize, sessionID)
					}

					metadata.Events = append(metadata.Events, &pb.SessionRecordingEvent{
						StartOffset: durationpb.New(e.Time.Sub(startTime)),
						Event: &pb.SessionRecordingEvent_Resize{
							Resize: &pb.SessionRecordingResizeEvent{
								Cols: int32(cols),
								Rows: int32(rows),
							},
						},
					})

					vt.Resize(cols, rows)
				}

			case *apievents.SessionEnd:
				if !lastActivityTime.IsZero() && e.Time.Sub(lastActivityTime) > inactivityThreshold {
					addInactivityEvent(lastActivityTime, e.Time)
				}

				recordThumbnail(e.EndTime)

			case *apievents.SessionJoin:
				activeUsers[e.User] = e.Time.Sub(startTime)

			case *apievents.SessionLeave:
				if joinTime, ok := activeUsers[e.User]; ok {
					metadata.Events = append(metadata.Events, &pb.SessionRecordingEvent{
						StartOffset: durationpb.New(joinTime),
						EndOffset:   durationpb.New(e.Time.Sub(startTime)),
						Event: &pb.SessionRecordingEvent_Join{
							Join: &pb.SessionRecordingJoinEvent{
								User: e.User,
							},
						},
					})

					delete(activeUsers, e.User)
				}

			case *apievents.SessionPrint:
				if !lastActivityTime.IsZero() && e.Time.Sub(lastActivityTime) > inactivityThreshold {
					addInactivityEvent(lastActivityTime, e.Time)
				}

				if _, err := vt.Write(e.Data); err != nil {
					return trace.Errorf("failed to write data to terminal: %w", err)
				}

				if sampler.shouldCapture(e.Time) {
					recordThumbnail(e.Time)
				}

				lastActivityTime = e.Time

			case *apievents.SessionStart:
				lastActivityTime = e.Time
				startTime = e.Time

				cols, rows, err := parseTerminalSize(e.TerminalSize)
				if err != nil {
					return trace.Wrap(err, "failed to parse terminal size %q for session %v", e.TerminalSize, sessionID)
				}

				metadata.ClusterName = e.ClusterName

				metadata.StartCols = int32(cols)
				metadata.StartRows = int32(rows)

				vt.Resize(cols, rows)
			}

		case err := <-errors:
			if err != nil {
				return trace.Wrap(err)
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if lastEvent == nil {
		return trace.Errorf("could not find any events for session %v", sessionID)
	}

	// Finish off any remaining activity events
	for user, userStartOffset := range activeUsers {
		metadata.Events = append(metadata.Events, &pb.SessionRecordingEvent{
			StartOffset: durationpb.New(userStartOffset),
			EndOffset:   durationpb.New(lastEvent.GetTime().Sub(startTime)),
			Event: &pb.SessionRecordingEvent_Join{
				Join: &pb.SessionRecordingJoinEvent{
					User: user,
				},
			},
		})
	}

	metadata.Duration = durationpb.New(lastEvent.GetTime().Sub(startTime))
	metadata.StartTime = timestamppb.New(startTime)
	metadata.EndTime = timestamppb.New(lastEvent.GetTime())

	thumbnails := sampler.result()
	for _, t := range thumbnails {
		if t == nil || t.state == nil {
			continue
		}

		svg := terminal.VtStateToSvg(t.state)

		thumbnail := &pb.SessionRecordingThumbnail{
			Svg:         svg,
			Cols:        int32(t.state.Cols),
			Rows:        int32(t.state.Rows),
			CursorX:     int32(t.state.CursorX),
			CursorY:     int32(t.state.CursorY),
			StartOffset: durationpb.New(t.startOffset),
			EndOffset:   durationpb.New(t.endOffset),
		}

		frames = append(frames, thumbnail)
	}

	thumbnail := getRandomThumbnail(frames)

	if thumbnail != nil {
		b, err := proto.Marshal(thumbnail)
		if err != nil {
			return trace.Wrap(err)
		}

		path, err := s.uploadHandler.UploadThumbnail(ctx, sessionID, bytes.NewReader(b))
		if err != nil {
			return trace.Wrap(err)
		}

		s.logger.Info("Uploaded session recording thumbnail", "path", path)
	}

	path, err := s.uploadMetadata(ctx, sessionID, metadata, frames)
	if err != nil {
		return trace.Wrap(err)
	}

	s.logger.Info("Uploaded session recording metadata", "path", path)

	return nil
}

func (s *RecordingMetadataService) uploadMetadata(ctx context.Context, sessionID session.ID, metadata *pb.SessionRecordingMetadata, frames []*pb.SessionRecordingThumbnail) (string, error) {
	buf := &bytes.Buffer{}
	writer := bufio.NewWriter(buf)

	if _, err := protodelim.MarshalTo(writer, metadata); err != nil {
		return "", trace.Wrap(err)
	}

	for _, frame := range frames {
		if _, err := protodelim.MarshalTo(writer, frame); err != nil {
			return "", trace.Wrap(err)
		}
	}

	if err := writer.Flush(); err != nil {
		return "", trace.Wrap(err)
	}

	return s.uploadHandler.UploadMetadata(ctx, sessionID, buf)
}

func parseTerminalSize(size string) (cols, rows int, err error) {
	parts := strings.Split(size, ":")
	if len(parts) != 2 {
		return 0, 0, trace.BadParameter("invalid terminal size %q", size)
	}

	cols, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, trace.Wrap(err, "invalid number of columns %q", parts[0])
	}

	rows, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, trace.Wrap(err, "invalid number of rows %q", parts[1])
	}

	return cols, rows, nil
}

func getRandomThumbnail(thumbnails []*pb.SessionRecordingThumbnail) *pb.SessionRecordingThumbnail {
	if len(thumbnails) == 0 {
		return nil
	}

	randomIndex := rand.Intn(len(thumbnails))

	return thumbnails[randomIndex]
}
