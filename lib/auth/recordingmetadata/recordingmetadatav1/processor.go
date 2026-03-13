/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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
	"io"
	"log/slog"
	"math"
	"time"

	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/types/known/durationpb"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/recordingmetadata"
)

// recordingProcessor consumes audit events from a session recording and produces metadata and a representative
// thumbnail. Implementations are specific to a session type (e.g. TTY for now, desktop and database in the future).
type recordingProcessor interface {
	// handleEvent processes a single audit event and updates the processor's internal state. Returning an error aborts
	// metadata generation for the recording.
	handleEvent(event apievents.AuditEvent) error
	// collect finalizes the metadata and returns the session recording metadata along with the chosen thumbnail.
	// It should be called after all events have been processed.
	collect() (*pb.SessionRecordingMetadata, *pb.SessionRecordingThumbnail)
}

func newRecordingProcessor(writer io.WriteCloser, logger *slog.Logger, sessionType recordingmetadata.SessionType, duration time.Duration) recordingProcessor {
	base := baseRecordingProcessor{
		metadata:          &pb.SessionRecordingMetadata{},
		writer:            writer,
		logger:            logger,
		thumbnailInterval: calculateThumbnailInterval(duration, maxThumbnails),
		thumbnailTime:     getRandomThumbnailTime(duration),
	}

	if sessionType == recordingmetadata.SessionTypeTTY {
		return newTTYRecordingProcessor(base)
	}

	return &noopRecordingProcessor{}
}

// baseRecordingProcessor holds the shared state and helpers used by all session-type-specific processors, such as
// the writer for streaming thumbnail frames and the logic for selecting the best representative thumbnail.
type baseRecordingProcessor struct {
	metadata           *pb.SessionRecordingMetadata
	thumbnailGenerator thumbnailGenerator

	lastEvent        apievents.AuditEvent
	lastActivityTime time.Time
	startTime        time.Time

	thumbnailInterval time.Duration
	thumbnailTime     time.Duration
	lastThumbnailTime time.Time
	thumbnail         *pb.SessionRecordingThumbnail

	writer io.WriteCloser
	logger *slog.Logger
}

func (b *baseRecordingProcessor) captureThumbnailIfNeeded(eventTime time.Time) {
	if eventTime.Sub(b.lastThumbnailTime) < b.thumbnailInterval {
		return
	}

	b.lastThumbnailTime = eventTime

	thumbnail := b.thumbnailGenerator.produceThumbnail()
	thumbnail.StartOffset = durationpb.New(eventTime.Sub(b.startTime))
	thumbnail.EndOffset = durationpb.New(eventTime.Add(b.thumbnailInterval).Add(-1 * time.Millisecond).Sub(b.startTime))

	if _, err := protodelim.MarshalTo(b.writer, thumbnail); err != nil {
		// log the error but continue processing other thumbnails and the session metadata (metadata is more important)
		b.logger.WarnContext(context.Background(), "Failed to marshal thumbnail entry", "error", err)
	}

	if b.thumbnail == nil {
		b.thumbnail = thumbnail

		return
	}

	previousDiff := math.Abs(float64(b.thumbnailTime - b.thumbnail.StartOffset.AsDuration()))
	diff := math.Abs(float64(b.thumbnailTime - eventTime.Sub(b.startTime)))

	if diff < previousDiff {
		// this thumbnail is closer to the ideal thumbnail time, use it instead
		b.thumbnail = thumbnail
	}
}

type noopRecordingProcessor struct{}

func (n *noopRecordingProcessor) handleEvent(event apievents.AuditEvent) error {
	return nil
}

func (n *noopRecordingProcessor) collect() (*pb.SessionRecordingMetadata, *pb.SessionRecordingThumbnail) {
	return nil, nil
}
