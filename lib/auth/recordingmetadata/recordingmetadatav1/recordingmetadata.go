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
	"bytes"
	"context"
	"log/slog"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/hinshun/vt10x"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/player"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/web/ttyplayback"
)

type RecordingMetadataService struct {
	logger        *slog.Logger
	streamer      player.Streamer
	uploadHandler events.UploadHandler
}

type RecordingMetadataConfig struct {
	Streamer      player.Streamer
	UploadHandler events.UploadHandler
}

const (
	// inactivityThreshold is the duration after which an inactivity event is recorded.
	inactivityThreshold = 5 * time.Second

	// maxThumbnails is the maximum number of thumbnails to store in the session metadata.
	maxThumbnails = 1000
)

const (
	attrReverse = 1 << iota
	attrUnderline
	attrBold
	attrGfx
	attrItalic
	attrBlink
	attrWrap
)

func NewRecordingMetadata(cfg RecordingMetadataConfig) *RecordingMetadataService {
	return &RecordingMetadataService{
		streamer:      cfg.Streamer,
		uploadHandler: cfg.UploadHandler,
		logger:        slog.With(teleport.ComponentKey, "recording_metadata"),
	}
}

func (s *RecordingMetadataService) ProcessSessionRecording(ctx context.Context, sessionID session.ID) error {
	evts, errors := s.streamer.StreamSessionEvents(ctx, sessionID, 0)

	var startTime time.Time
	var lastEvent apievents.AuditEvent
	var lastActivityTime time.Time
	var nextThumbnailTime time.Time

	thumbnailInterval := 1 * time.Second
	activeUsers := make(map[string]int64)

	vt := vt10x.New()

	metadata := &pb.SessionRecordingMetadata{
		Thumbnails: make([]*pb.SessionRecordingThumbnail, 0),
		Events:     make([]*pb.SessionRecordingEvent, 0),
	}

	addInactivityEvent := func(start, end time.Time) {
		if start.IsZero() || end.IsZero() {
			return
		}

		inactivityStart := int64(start.Sub(startTime) / time.Millisecond)
		inactivityEnd := int64(end.Sub(startTime) / time.Millisecond)

		metadata.Events = append(metadata.Events, &pb.SessionRecordingEvent{
			StartTime: inactivityStart,
			EndTime:   inactivityEnd,
			Event: &pb.SessionRecordingEvent_Inactivity{
				Inactivity: &pb.SessionRecordingInactivityEvent{},
			},
		})
	}

	recordThumbnail := func(start, end time.Time) {
		state := vt.DumpState()

		svg := ttyplayback.TerminalStateToSVG(state)

		thumbnailStart := int64(start.Sub(startTime) / time.Millisecond)
		thumbnailEnd := int64(end.Sub(startTime) / time.Millisecond)

		metadata.Thumbnails = append(metadata.Thumbnails, &pb.SessionRecordingThumbnail{
			Svg:       svg,
			Cols:      int32(state.Cols),
			Rows:      int32(state.Rows),
			CursorX:   int32(state.CursorX),
			CursorY:   int32(state.CursorY),
			StartTime: thumbnailStart,
			EndTime:   thumbnailEnd,
		})
	}

loop:
	for {
		select {
		case evt, more := <-evts:
			if !more {
				break loop
			}

			lastEvent = evt

			switch e := evt.(type) {
			case *apievents.DatabaseSessionStart:
			case *apievents.WindowsDesktopSessionStart:
				// Unsupported session recording type
				return nil

			case *apievents.Resize:
				parts := strings.Split(e.TerminalSize, ":")

				if len(parts) == 2 {
					cols, rows, err := parseTerminalSize(e.TerminalSize)
					if err != nil {
						return trace.Wrap(err, "failed to parse terminal size %q for session %v", e.TerminalSize, sessionID)
					}

					metadata.Events = append(metadata.Events, &pb.SessionRecordingEvent{
						StartTime: int64(e.Time.Sub(startTime) / time.Millisecond),
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
				metadata.Duration = int64(e.EndTime.Sub(e.StartTime) / time.Millisecond)

				if !lastActivityTime.IsZero() && e.Time.Sub(lastActivityTime) > inactivityThreshold {
					addInactivityEvent(lastActivityTime, e.Time)
				}

				recordThumbnail(e.EndTime, e.EndTime)

				endTime := int64(e.EndTime.Sub(startTime) / time.Millisecond)

				for user, startTime := range activeUsers {
					metadata.Events = append(metadata.Events, &pb.SessionRecordingEvent{
						StartTime: startTime,
						EndTime:   endTime,
						Event: &pb.SessionRecordingEvent_Join{
							Join: &pb.SessionRecordingJoinEvent{
								User: user,
							},
						},
					})
				}

			case *apievents.SessionJoin:
				activeUsers[e.User] = int64(e.Time.Sub(startTime) / time.Millisecond)

			case *apievents.SessionLeave:
				if joinTime, ok := activeUsers[e.User]; ok {
					metadata.Events = append(metadata.Events, &pb.SessionRecordingEvent{
						StartTime: joinTime,
						EndTime:   int64(e.Time.Sub(startTime) / time.Millisecond),
						Event: &pb.SessionRecordingEvent_Join{
							Join: &pb.SessionRecordingJoinEvent{
								User: e.User,
							},
						},
					})

					delete(activeUsers, e.User)
				}

			case *apievents.SessionPrint:
				if lastActivityTime != (time.Time{}) && e.Time.Sub(lastActivityTime) > inactivityThreshold {
					addInactivityEvent(lastActivityTime, e.Time)
				}

				if _, err := vt.Write(e.Data); err != nil {
					return trace.Errorf("failed to write data to terminal: %w", err)
				}

				if e.Time.After(nextThumbnailTime) {
					startTime := e.Time
					endTime := e.Time.Add(thumbnailInterval).Add(-1 * time.Millisecond)

					recordThumbnail(startTime, endTime)

					nextThumbnailTime = e.Time.Add(thumbnailInterval)
				}

				lastActivityTime = e.Time

			case *apievents.SessionStart:
				lastActivityTime = e.Time
				startTime = e.Time

				cols, rows, err := parseTerminalSize(e.TerminalSize)
				if err != nil {
					return trace.Wrap(err, "failed to parse terminal size %q for session %v", e.TerminalSize, sessionID)
				}

				metadata.StartCols = int32(cols)
				metadata.StartRows = int32(rows)

				vt.Resize(cols, rows)
			}

		case err := <-errors:
			return trace.Wrap(err)
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if lastEvent == nil {
		return trace.Errorf("could not find any events for session %v", sessionID)
	}

	if metadata.Duration == 0 {
		metadata.Duration = int64(lastEvent.GetTime().Sub(startTime) / time.Millisecond)
	}

	thumbnail := getRandomThumbnail(metadata.Thumbnails)
	metadata.Thumbnails = getEvenlySampledThumbnails(metadata.Thumbnails, maxThumbnails)

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

	b, err := proto.Marshal(metadata)
	if err != nil {
		return trace.Wrap(err)
	}

	path, err := s.uploadHandler.UploadMetadata(ctx, sessionID, bytes.NewReader(b))
	if err != nil {
		return trace.Wrap(err)
	}

	s.logger.Info("Uploaded session recording metadata", "path", path)

	return nil
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

func getEvenlySampledThumbnails(thumbnails []*pb.SessionRecordingThumbnail, maxItems int) []*pb.SessionRecordingThumbnail {
	if len(thumbnails) <= maxItems {
		return thumbnails
	}

	result := make([]*pb.SessionRecordingThumbnail, 0, maxItems)
	step := float64(len(thumbnails)) / float64(maxItems)

	for i := 0; i < maxItems && int(float64(i)*step) < len(thumbnails); i++ {
		result = append(result, thumbnails[int(float64(i)*step)])
	}

	return result
}
