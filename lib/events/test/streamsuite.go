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

package test

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/session"
)

type flakyHandler struct {
	events.MultipartHandler
	mu          sync.Mutex
	shouldFlake bool
	flakedParts map[int64]bool
}

func newFlakyHandler(handler events.MultipartHandler) *flakyHandler {
	return &flakyHandler{
		MultipartHandler: handler,
		flakedParts:      make(map[int64]bool),
	}
}

func (f *flakyHandler) UploadPart(ctx context.Context, upload events.StreamUpload, partNumber int64, partBody io.ReadSeeker) (*events.StreamPart, error) {
	var shouldFlake bool
	f.mu.Lock()
	if f.shouldFlake && !f.flakedParts[partNumber] {
		shouldFlake = true
		f.flakedParts[partNumber] = true
	}
	f.mu.Unlock()

	if shouldFlake {
		return nil, trace.Errorf("flakeity flake flake")
	}

	return f.MultipartHandler.UploadPart(ctx, upload, partNumber, partBody)
}

func (f *flakyHandler) setFlakeUpload(flake bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shouldFlake = flake
}

// StreamParams configures parameters of a stream test suite
type StreamParams struct {
	// PrintEvents is amount of print events to generate
	PrintEvents int64
	// ConcurrentUploads is amount of concurrent uploads
	ConcurrentUploads int
	// MinUploadBytes is minimum required upload bytes
	MinUploadBytes int64
	// Flaky is a flag that indicates that the handler should be flaky
	Flaky bool
	// ForceFlush is a flag that indicates that the handler should be forced to flush
	// partially filled slices during event input.
	ForceFlush bool
}

// StreamSinglePart tests stream upload and subsequent download and reads the results
func StreamSinglePart(t *testing.T, handler events.MultipartHandler) {
	StreamWithPermutedParameters(t, handler, StreamParams{
		PrintEvents:    1024,
		MinUploadBytes: 1024 * 1024,
	})
}

// StreamWithPadding tests stream upload in a case where significant padding must be added. Note that
// in practice padding is only necessarily added in the 'ForceFlush' permutation as single-slice uploads
// do not require padding.
func StreamWithPadding(t *testing.T, handler events.MultipartHandler) {
	StreamWithPermutedParameters(t, handler, StreamParams{
		PrintEvents:    10,
		MinUploadBytes: 1024 * 1024,
	})
}

// Stream tests stream upload and subsequent download and reads the results
func Stream(t *testing.T, handler events.MultipartHandler) {
	StreamWithPermutedParameters(t, handler, StreamParams{
		PrintEvents:       1024,
		MinUploadBytes:    1024,
		ConcurrentUploads: 2,
	})
}

// StreamManyParts tests stream upload and subsequent download and reads the results
func StreamManyParts(t *testing.T, handler events.MultipartHandler) {
	StreamWithPermutedParameters(t, handler, StreamParams{
		PrintEvents:       8192,
		MinUploadBytes:    1024,
		ConcurrentUploads: 64,
	})
}

// StreamResumeManyParts tests stream upload, failure to complete, resuming
// and subsequent download and reads the results.
func StreamResumeManyParts(t *testing.T, handler events.MultipartHandler) {
	StreamResumeWithParameters(t, handler, StreamParams{
		PrintEvents:       8192,
		MinUploadBytes:    1024,
		ConcurrentUploads: 64,
	})
}

// StreamWithPermutedParameters tests stream upload and subsequent download and reads the results, repeating
// the process with various permutations of flake and flush parameters in order to better cover padding and
// retry logic, which are easy to accidentally fail to cover.
func StreamWithPermutedParameters(t *testing.T, handler events.MultipartHandler, params StreamParams) {
	cases := []struct{ Flaky, ForceFlush bool }{
		{Flaky: false, ForceFlush: false},
		{Flaky: true, ForceFlush: false},
		{Flaky: false, ForceFlush: true},
		{Flaky: true, ForceFlush: true},
	}

	for _, cc := range cases {
		t.Run(fmt.Sprintf("Flaky=%v,ForceFlush=%v", cc.Flaky, cc.ForceFlush), func(t *testing.T) {
			pc := params
			pc.Flaky = cc.Flaky
			pc.ForceFlush = cc.ForceFlush
			StreamWithParameters(t, handler, pc)
		})
	}
}

// StreamEmpty verifies stream upload with zero events gets correctly discarded. This behavior is
// necessary in order to prevent a bug where agents might think they have failed to create a multipart
// upload and create a new one, resulting in duplicate recordings overwriiting each other.
func StreamEmpty(t *testing.T, handler events.MultipartHandler) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sid := session.NewID()

	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader:          handler,
		MinUploadBytes:    1024,
		ConcurrentUploads: 2,
	})
	require.NoError(t, err)

	stream, err := streamer.CreateAuditStream(ctx, sid)
	require.NoError(t, err)

	select {
	case status := <-stream.Status():
		require.Equal(t, status.LastEventIndex, int64(-1))
	case <-time.After(time.Second):
		t.Fatalf("Timed out waiting for status update.")
	}

	require.NoError(t, stream.Complete(ctx))

	f, err := os.CreateTemp("", string(sid))
	require.NoError(t, err)
	defer os.Remove(f.Name())

	require.True(t, trace.IsNotFound(handler.Download(ctx, sid, f)))
}

// StreamWithParameters tests stream upload and subsequent download and reads the results
func StreamWithParameters(t *testing.T, handler events.MultipartHandler, params StreamParams) {
	ctx := context.TODO()

	inEvents := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: params.PrintEvents})
	sid := session.ID(inEvents[0].(events.SessionMetadataGetter).GetSessionID())

	forceFlush := make(chan struct{})

	wrappedHandler := newFlakyHandler(handler)

	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader:          wrappedHandler,
		MinUploadBytes:    params.MinUploadBytes,
		ConcurrentUploads: params.ConcurrentUploads,
		ForceFlush:        forceFlush,
		RetryConfig: &retryutils.LinearConfig{
			Step: time.Millisecond * 10,
			Max:  time.Millisecond * 500,
		},
	})
	require.NoError(t, err)

	stream, err := streamer.CreateAuditStream(ctx, sid)
	require.NoError(t, err)

	select {
	case status := <-stream.Status():
		require.Equal(t, status.LastEventIndex, int64(-1))
	case <-time.After(time.Second):
		t.Fatalf("Timed out waiting for status update.")
	}

	// if enabled, flake causes the first upload attempt for each multipart upload part
	// to fail. necessary in order to cover upload retry logic, which has historically been
	// a source of bugs.
	wrappedHandler.setFlakeUpload(params.Flaky)

	timeout := time.After(time.Minute)

	for i, event := range inEvents {
		if params.ForceFlush && i%(len(inEvents)/3) == 0 {
			select {
			case forceFlush <- struct{}{}:
			case <-timeout:
				t.Fatalf("Timed out waiting for force flush.")
			}
		}
		err := stream.RecordEvent(ctx, eventstest.PrepareEvent(event))
		require.NoError(t, err)
	}

	err = stream.Complete(ctx)
	require.NoError(t, err)

	f, err := os.CreateTemp("", string(sid))
	require.NoError(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = handler.Download(ctx, sid, f)
	require.NoError(t, err)

	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	reader, err := events.NewProtoReader(f, nil)
	require.NoError(t, err)

	out, err := reader.ReadAll(ctx)
	require.NoError(t, err)

	stats := reader.GetStats()
	require.Equal(t, int64(0), stats.SkippedEvents)
	require.Equal(t, int64(0), stats.OutOfOrderEvents)
	require.Equal(t, int64(len(inEvents)), stats.TotalEvents)

	require.Equal(t, inEvents, out)
}

// StreamResumeWithParameters expects initial complete attempt to fail
// but subsequent resume to succeed
func StreamResumeWithParameters(t *testing.T, handler events.MultipartHandler, params StreamParams) {
	ctx := context.TODO()

	inEvents := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: params.PrintEvents})
	sid := session.ID(inEvents[0].(events.SessionMetadataGetter).GetSessionID())

	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader:          handler,
		MinUploadBytes:    params.MinUploadBytes,
		ConcurrentUploads: params.ConcurrentUploads,
		RetryConfig: &retryutils.LinearConfig{
			Step: time.Millisecond * 10,
			Max:  time.Millisecond * 500,
		},
	})
	require.NoError(t, err)

	upload, err := handler.CreateUpload(ctx, sid)
	require.NoError(t, err)

	stream, err := streamer.CreateAuditStreamForUpload(ctx, sid, *upload)
	require.NoError(t, err)

	for _, event := range inEvents {
		err := stream.RecordEvent(ctx, eventstest.PrepareEvent(event))
		require.NoError(t, err)
	}

	err = stream.Complete(ctx)
	require.Error(t, err, "First complete attempt should fail here.")

	stream, err = streamer.ResumeAuditStream(ctx, sid, upload.ID)
	require.NoError(t, err)

	// First update always starts with -1 and indicates
	// that resume has been started successfully
	select {
	case status := <-stream.Status():
		require.Equal(t, status.LastEventIndex, int64(-1))
	case <-time.After(time.Second):
		t.Fatalf("Timed out waiting for status update.")
	}

	err = stream.Complete(ctx)
	require.NoError(t, err, "Complete after resume should succeed")

	f, err := os.CreateTemp("", string(sid))
	require.NoError(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = handler.Download(ctx, sid, f)
	require.NoError(t, err)

	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	reader, err := events.NewProtoReader(f, nil)
	require.NoError(t, err)

	out, err := reader.ReadAll(ctx)
	require.NoError(t, err)

	stats := reader.GetStats()
	require.Equal(t, int64(0), stats.SkippedEvents)
	require.Equal(t, int64(0), stats.OutOfOrderEvents)
	require.Equal(t, int64(len(inEvents)), stats.TotalEvents)

	require.Equal(t, inEvents, out)
}
