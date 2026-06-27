/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package events_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/session"
)

// TestProtoStreamPartUploadRetryExhaustion verifies that when a part upload
// fails after exhausting all retries, the stream completion returns an error
// rather than silently succeeding with a corrupted recording.
// This is a regression test for https://github.com/gravitational/teleport/issues/65895
func TestProtoStreamPartUploadRetryExhaustion(t *testing.T) {
	ctx := context.Background()

	uploadPartCalls := 0
	uploader := &eventstest.MockUploader{
		UploadPartError: errors.New("s3 persistent error"),
		MockUploadPart: func(ctx context.Context, upload events.StreamUpload, partNumber int64, partBody io.ReadSeeker) (*events.StreamPart, error) {
			uploadPartCalls++
			return nil, errors.New("s3 persistent error")
		},
	}

	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader:       uploader,
		MinUploadBytes: 1, // Force immediate upload after each event to avoid long test runtime
		RetryConfig:    retryutils.RetryConfig{First: 1, Step: 1, Max: 1}, // Short-circuit retries for test speed
	})
	require.NoError(t, err)

	evts := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 5})
	sid := session.ID(evts[0].(events.SessionMetadataGetter).GetSessionID())

	stream, err := streamer.CreateAuditStream(ctx, sid)
	require.NoError(t, err)

	for _, evt := range evts {
		prepared, err := events.NewPreparer(events.PreparerConfig{
			SessionID:   sid,
			Namespace:   "default",
			ClusterName: "test-cluster",
		})
		require.NoError(t, err)
		preparedEvent, err := prepared.PrepareSessionEvent(evt)
		require.NoError(t, err)
		require.NoError(t, stream.RecordEvent(ctx, preparedEvent))
	}

	// Complete should fail because parts could not be uploaded
	err = stream.Complete(ctx)
	require.Error(t, err)
	require.True(t, trace.IsLimitExceeded(err), "expected LimitExceeded error indicating retry exhaustion, got: %v", err)
}

// TestProtoStreamPartialPartFailure verifies that when some parts upload
// successfully but others fail after retry exhaustion, Complete returns an
// error and does not emit a misleading session.upload event.
func TestProtoStreamPartialPartFailure(t *testing.T) {
	ctx := context.Background()

	uploader := &eventstest.MockUploader{
		MockUploadPart: func(ctx context.Context, upload events.StreamUpload, partNumber int64, partBody io.ReadSeeker) (*events.StreamPart, error) {
			// First part succeeds, subsequent parts fail
			if partNumber == 1 {
				return &events.StreamPart{Number: partNumber}, nil
			}
			return nil, errors.New("s3 persistent error")
		},
	}

	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader:       uploader,
		MinUploadBytes: 1, // Force immediate upload after each event
		RetryConfig:    retryutils.RetryConfig{First: 1, Step: 1, Max: 1}, // Short-circuit retries for test speed
	})
	require.NoError(t, err)

	evts := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 5})
	sid := session.ID(evts[0].(events.SessionMetadataGetter).GetSessionID())

	stream, err := streamer.CreateAuditStream(ctx, sid)
	require.NoError(t, err)

	preparer, err := events.NewPreparer(events.PreparerConfig{
		SessionID:   sid,
		Namespace:   "default",
		ClusterName: "test-cluster",
	})
	require.NoError(t, err)

	for _, evt := range evts {
		preparedEvent, err := preparer.PrepareSessionEvent(evt)
		require.NoError(t, err)
		require.NoError(t, stream.RecordEvent(ctx, preparedEvent))
	}

	// Complete should fail because not all parts uploaded successfully
	err = stream.Complete(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to upload")
}

// TestProtoStreamUploadFailurePreservesLocalRecording verifies that when
// stream.Complete fails due to part upload exhaustion, the error is properly
// propagated so that the agent does not delete the local recording file.
func TestProtoStreamUploadFailurePreservesLocalRecording(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	uploader := &eventstest.MockUploader{
		MockUploadPart: func(ctx context.Context, upload events.StreamUpload, partNumber int64, partBody io.ReadSeeker) (*events.StreamPart, error) {
			return nil, errors.New("persistent storage backend error")
		},
	}

	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader:    uploader,
		RetryConfig: retryutils.RetryConfig{First: 1, Step: 1, Max: 1}, // Short-circuit retries for test speed
	})
	require.NoError(t, err)

	sid := session.NewID()
	stream, err := streamer.CreateAuditStream(ctx, sid)
	require.NoError(t, err)

	// Record a few events to create slices that need uploading
	preparer, err := events.NewPreparer(events.PreparerConfig{
		SessionID:   sid,
		Namespace:   "default",
		ClusterName: "test-cluster",
	})
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		evt := &apievents.SessionPrint{
			Metadata: apievents.Metadata{
				Type:  apievents.SessionPrintEvent,
				Time:  time.Now(),
				Index: int64(i),
			},
			SessionMetadata: apievents.SessionMetadata{SessionID: string(sid)},
			Data:            []byte("test data"),
		}
		prepared, err := preparer.PrepareSessionEvent(evt)
		require.NoError(t, err)
		require.NoError(t, stream.RecordEvent(ctx, prepared))
	}

	// Complete should fail - this error should prevent the agent from
	// deleting the local recording file
	err = stream.Complete(ctx)
	require.Error(t, err)
	require.True(t, trace.IsLimitExceeded(err) || trace.IsConnectionProblem(err),
		"expected a retry-exhaustion or connection error, got: %T %v", err, err)
}
