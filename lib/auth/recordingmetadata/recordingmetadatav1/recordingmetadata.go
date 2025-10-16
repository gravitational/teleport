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
	"github.com/hinshun/vt10x"
	"golang.org/x/sync/semaphore"
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
	logger             *slog.Logger
	streamer           player.Streamer
	uploadHandler      UploadHandler
	concurrencyLimiter *semaphore.Weighted
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

	// concurrencyLimit limits the number of concurrent processing operations (matches the session summarizer).
	concurrencyLimit = 150
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
		streamer:           cfg.Streamer,
		uploadHandler:      cfg.UploadHandler,
		logger:             slog.With(teleport.ComponentKey, "recording_metadata"),
		concurrencyLimiter: semaphore.NewWeighted(concurrencyLimit),
	}, nil
}

// ProcessSessionRecording processes the session recording associated with the provided session ID.
// It streams session events, generates metadata, and uploads thumbnails and metadata.
func (s *RecordingMetadataService) ProcessSessionRecording(ctx context.Context, sessionID session.ID) error {
	sessionsPendingMetric.Inc()

	if err := s.concurrencyLimiter.Acquire(ctx, 1); err != nil {
		sessionsPendingMetric.Dec()
		return trace.Wrap(err)
	}
	defer s.concurrencyLimiter.Release(1)

	sessionsPendingMetric.Dec()

	sessionsProcessingMetric.Inc()
	defer sessionsProcessingMetric.Dec()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	evts, errors := s.streamer.StreamSessionEvents(ctx, sessionID, 0)

	var startTime time.Time
	var lastEvent apievents.AuditEvent
	var lastActivityTime time.Time

	thumbnailInterval := 1 * time.Second
	activeUsers := make(map[string]time.Duration)

	vt := vt10x.New()

	metadata := &pb.SessionRecordingMetadata{}

	addInactivityEvent := func(start, end time.Time) {
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
		cols, rows := vt.Size()

		sampler.add(&thumbnailState{
			svg:           terminal.VtToSvg(vt),
			cols:          cols,
			rows:          rows,
			cursorVisible: vt.CursorVisible(),
			cursor:        vt.Cursor(),
		}, start)
	}

	var hasSeenPrintEvent bool

loop:
	for {
		select {
		case evt, ok := <-evts:
			if !ok {
				break loop
			}

			lastEvent = evt

			switch e := evt.(type) {
			case *apievents.DatabaseSessionStart, *apievents.WindowsDesktopSessionStart:
				// Unsupported session recording types
				return nil

			case *apievents.Resize:
				size, err := session.UnmarshalTerminalParams(e.TerminalSize)
				if err != nil {
					return trace.Wrap(err, "parsing terminal size %q for session %v", e.TerminalSize, sessionID)
				}

				// if we haven't seen a print event yet, update the starting size to the latest resize
				// this handles cases where the initial terminal size is not 80x24 and is resized immediately
				// before any output is printed
				if !hasSeenPrintEvent {
					metadata.StartCols = int32(size.W)
					metadata.StartRows = int32(size.H)
				}

				metadata.Events = append(metadata.Events, &pb.SessionRecordingEvent{
					StartOffset: durationpb.New(e.Time.Sub(startTime)),
					Event: &pb.SessionRecordingEvent_Resize{
						Resize: &pb.SessionRecordingResizeEvent{
							Cols: int32(size.W),
							Rows: int32(size.H),
						},
					},
				})

				vt.Resize(size.W, size.H)

			case *apievents.SessionEnd:
				if !lastActivityTime.IsZero() && e.Time.Sub(lastActivityTime) > inactivityThreshold {
					addInactivityEvent(lastActivityTime, e.Time)
				}

				if sampler.shouldCapture(e.Time) {
					recordThumbnail(e.Time)
				}

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
				// mark that we've seen the first print event so we don't update the starting size anymore
				if !hasSeenPrintEvent {
					hasSeenPrintEvent = true
				}

				if !lastActivityTime.IsZero() && e.Time.Sub(lastActivityTime) > inactivityThreshold {
					addInactivityEvent(lastActivityTime, e.Time)
				}

				if _, err := vt.Write(e.Data); err != nil {
					return trace.Errorf("writing data to terminal: %w", err)
				}

				if sampler.shouldCapture(e.Time) {
					recordThumbnail(e.Time)
				}

				lastActivityTime = e.Time

			case *apievents.SessionStart:
				lastActivityTime = e.Time
				startTime = e.Time

				size, err := session.UnmarshalTerminalParams(e.TerminalSize)
				if err != nil {
					return trace.Wrap(err, "parsing terminal size %q for session %v", e.TerminalSize, sessionID)
				}

				// store the initial terminal size, this is typically 80:24 and is resized immediately
				metadata.StartCols = int32(size.W)
				metadata.StartRows = int32(size.H)

				metadata.ClusterName = e.ClusterName
				metadata.User = e.User

				switch e.Protocol {
				case events.EventProtocolSSH:
					metadata.ResourceName = e.ServerHostname
					metadata.Type = pb.SessionRecordingType_SESSION_RECORDING_TYPE_SSH

				case events.EventProtocolKube:
					metadata.ResourceName = e.KubernetesCluster
					metadata.Type = pb.SessionRecordingType_SESSION_RECORDING_TYPE_KUBERNETES
				}

				vt.Resize(size.W, size.H)
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
		return trace.NotFound("no events found for session %v", sessionID)
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

	if err := s.upload(ctx, sessionID, metadata, thumbnails); err != nil {
		sessionsProcessedMetric.WithLabelValues( /* success */ "false").Inc()

		return trace.Wrap(err)
	}

	sessionsProcessedMetric.WithLabelValues( /* success */ "true").Inc()

	return nil
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
		Svg:           t.state.svg,
		Cols:          int32(t.state.cols),
		Rows:          int32(t.state.rows),
		CursorX:       int32(t.state.cursor.X),
		CursorY:       int32(t.state.cursor.Y),
		CursorVisible: t.state.cursorVisible,
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
