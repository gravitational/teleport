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
	"errors"
	"io"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/semaphore"
	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	"github.com/gravitational/teleport/lib/auth/recordingencryption"
	"github.com/gravitational/teleport/lib/auth/recordingmetadata"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/player"
	"github.com/gravitational/teleport/lib/session"
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
	encrypter          events.EncryptionWrapper
}

// RecordingMetadataServiceConfig defines the configuration for the RecordingMetadataService.
type RecordingMetadataServiceConfig struct {
	// Streamer is used to stream session events.
	Streamer player.Streamer
	// UploadHandler is used to upload session metadata and thumbnails.
	UploadHandler UploadHandler
	// Encrypter is used to encrypt session metadata and thumbnails.
	Encrypter events.EncryptionWrapper
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
		encrypter:          cfg.Encrypter,
	}, nil
}

// ProcessSessionRecording processes the session recording associated with the provided session ID.
// It streams session events, generates metadata, and uploads thumbnails and metadata.
func (s *RecordingMetadataService) ProcessSessionRecording(ctx context.Context, sessionID session.ID, sessionType recordingmetadata.SessionType, duration time.Duration) error {
	if sessionType == recordingmetadata.SessionTypeUnspecified {
		return nil
	}

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

	// will either finish the upload or cancel it if exited early
	var finish sync.Once

	w, cancelUpload, uploadErrs := s.startUpload(ctx, sessionID)
	select {
	case err := <-uploadErrs:
		return trace.Wrap(err)
	default:
	}
	defer func() {
		finish.Do(func() {
			cancelUpload()
			w.Close()
		})
	}()

	processor := newRecordingProcessor(w, s.logger.With("session_id", sessionID), sessionType, duration)
	defer processor.release()

	evts, errors := s.streamer.StreamSessionEvents(ctx, sessionID, 0)

loop:
	for {
		select {
		case evt, ok := <-evts:
			if !ok {
				break loop
			}

			if err := processor.handleEvent(evt); err != nil {
				return trace.Wrap(err)
			}

		case err := <-uploadErrs:
			return trace.Wrap(err)

		case err := <-errors:
			if err != nil {
				return trace.Wrap(err)
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}

	metadata, thumbnail := processor.collect()
	if metadata == nil {
		return trace.NotFound("no events found for session %v", sessionID)
	}

	if thumbnail != nil {
		if err := s.uploadThumbnail(ctx, sessionID, thumbnail); err != nil {
			s.logger.WarnContext(ctx, "Failed to upload thumbnail",
				"session_id", sessionID, "error", err)
		}
	}

	if _, err := protodelim.MarshalTo(w, metadata); err != nil {
		return trace.Wrap(err)
	}

	var err error

	finish.Do(func() {
		err = w.Close()

		if err == nil {
			err = <-uploadErrs
		}
	})

	if err != nil {
		sessionsProcessedMetric.WithLabelValues( /* success */ "false").Inc()

		return trace.Wrap(err)
	}

	sessionsProcessedMetric.WithLabelValues( /* success */ "true").Inc()

	return nil
}

func (s *RecordingMetadataService) startUpload(ctx context.Context, sessionID session.ID) (io.WriteCloser, context.CancelFunc, <-chan error) {
	uploadCtx, cancel := context.WithCancel(ctx)
	r, w := io.Pipe()
	errs := make(chan error, 1)
	var writer io.WriteCloser = w
	if s.encrypter != nil {
		// wrap the pipe writer with encryption
		// WithEncryption will never close the underlying writer when the returned
		// WriteCloser is closed, so we need to create a multiCloser to close both
		// the encrypted writer and the pipe writer.
		encrypted, err := s.encrypter.WithEncryption(uploadCtx, w)
		switch {
		case err == nil:
			writer = &multiCloser{
				WriteCloser: encrypted,
				pipeCloser:  w,
			}
		case errors.Is(err, recordingencryption.ErrEncryptionDisabled):
			// if encryption isn't enabled, do nothing
		default:
			cancel()
			errs <- trace.Wrap(err, "starting recording encrypter")
			return nil, nil, errs
		}
	}
	go func() {
		defer r.Close()

		path, err := s.uploadHandler.UploadMetadata(uploadCtx, sessionID, r)
		if err != nil {
			errs <- trace.Wrap(err)
			return
		}

		s.logger.DebugContext(ctx, "Uploaded session recording metadata", "path", path)
		errs <- nil
	}()

	return writer, cancel, errs
}

// multiCloser is an io.WriteCloser that closes the underlying
// WriteCloser and an additional Closer.
type multiCloser struct {
	io.WriteCloser
	pipeCloser io.Closer
}

func (m *multiCloser) Close() error {
	// flush the encryption writer and close the pipe
	errEncryption := m.WriteCloser.Close()
	errPipe := m.pipeCloser.Close()
	return trace.NewAggregate(errEncryption, errPipe)
}

func (s *RecordingMetadataService) uploadThumbnail(ctx context.Context, sessionID session.ID, thumbnail *pb.SessionRecordingThumbnail) error {
	if thumbnail == nil {
		return nil
	}

	b, err := proto.Marshal(thumbnail)
	if err != nil {
		return trace.Wrap(err)
	}

	var buf io.Reader = bytes.NewReader(b)
	if s.encrypter != nil {
		writeBuffer := bytes.NewBuffer(nil)
		encryptedWriter, err := s.encrypter.WithEncryption(ctx, &nopCloser{writeBuffer})
		switch {
		case err == nil:
			if _, err := io.Copy(encryptedWriter, buf); err != nil {
				encryptedWriter.Close()
				return trace.Wrap(err)
			}
			if err := encryptedWriter.Close(); err != nil {
				return trace.Wrap(err)
			}
			buf = writeBuffer
		case errors.Is(err, recordingencryption.ErrEncryptionDisabled):
			// if encryption isn't enabled, do nothing
		default:
			return trace.Wrap(err, "starting recording encrypter")
		}
	}

	path, err := s.uploadHandler.UploadThumbnail(ctx, sessionID, buf)
	if err != nil {
		return trace.Wrap(err)
	}

	s.logger.DebugContext(ctx, "Uploaded session recording thumbnail", "path", path)

	return nil
}

type nopCloser struct {
	io.Writer
}

func (n nopCloser) Close() error {
	return nil
}

// getRandomThumbnailTime returns the ideal time offset for capturing a thumbnail
// within the session duration based on the provided interval.
// It avoids the first and last 20% of the session recording to increase the chances of
// getting a thumbnail with meaningful content.
func getRandomThumbnailTime(duration time.Duration) time.Duration {
	minIndex := int(0.2 * float64(duration))
	maxIndex := int(0.8 * float64(duration))

	if maxIndex <= minIndex {
		return duration / 2
	}

	return time.Duration(rand.IntN(maxIndex-minIndex) + minIndex)
}

func calculateThumbnailInterval(duration time.Duration, maxThumbnails int) time.Duration {
	interval := time.Second

	if duration > time.Duration(maxThumbnails)*time.Second {
		interval = duration / time.Duration(maxThumbnails)
	}

	interval = interval.Round(time.Second)

	if interval < time.Second {
		interval = time.Second
	}

	return interval
}
