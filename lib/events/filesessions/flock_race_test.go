/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package filesessions

import (
	"bytes"
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/session"
)

// TestCompleteUploadTearsConcurrentUpload reproduces a data-loss race between
// the async uploader and CompleteUpload.
//
// Both operate on the same <sid>.tar. lib/service/service.go passes one
// uploadsDir as the uploader's ScanDir and as the filesessions Handler's
// Directory, and NewStreamer does the same thing (Handler{Directory: cfg.Dir}),
// so this is the production wiring, not a contrivance.
//
// The uploader opens the recording, flocks it, and streams it to the audit
// backend:
//
//	sessionFile, err := os.OpenFile(sessionFilePath, os.O_RDWR, 0)   // fileasync.go
//	unlock, err := utils.FSTryWriteLock(sessionFilePath)
//
// CompleteUpload truncates that same file BEFORE it asks for the lock:
//
//	// Prevent other processes from accessing this file until the write is completed
//	f, err := GetOpenFileFunc()(uploadPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)  // filestream.go
//	unlock, err := utils.FSTryWriteLock(uploadPath)
//
// O_TRUNC is a side effect of open(2) and takes no lock, and flock is advisory,
// so holding the lock does not stop the truncate. The comment describes an
// intent the code does not implement.
//
// The window this exploits is the UPLOAD DURATION, which for a real audit
// backend is long. The test does not inject any delay into production code: it
// gates the Streamer, which is a supported configuration point and is exactly
// how a slow or remote backend behaves.
//
// Expected on the current code: the uploader either fails with a truncated
// read, or silently streams a partial recording. Both are session recording
// loss.
func TestCompleteUploadTearsConcurrentUpload(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	gate := newGatedStreamer()

	p := newUploaderPack(ctx, t, uploaderPackConfig{
		wrapProtoStreamer: func(s events.Streamer) (events.Streamer, error) {
			gate.inner = s
			return gate, nil
		},
	})

	clock, ok := p.clock.(*clockwork.FakeClock)
	require.True(t, ok, "fake clock expected")
	clock.BlockUntilContext(ctx, 1)

	// Write a complete recording. This runs the normal streaming path and ends
	// in CompleteUpload, leaving a finished <sid>.tar in the scan dir.
	inEvents := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 4096})
	sid := session.ID(inEvents[0].(events.SessionMetadataGetter).GetSessionID())
	p.emitEvents(ctx, t, inEvents)

	recordingPath := p.scanDir + "/" + sid.String() + tarExt
	completeSize := fileSize(t, recordingPath)
	require.NotZero(t, completeSize, "recording should exist before the uploader runs")

	// Start the scan, which picks up the recording and begins uploading it.
	clock.Advance(p.scanPeriod + time.Second)

	// Wait until the uploader is genuinely mid-upload: it has opened the file,
	// taken the flock, read at least one event, and is now blocked handing that
	// event to the audit backend. This is the state a slow backend leaves it in
	// for the whole upload.
	select {
	case <-gate.reached:
	case <-ctx.Done():
		t.Fatal("timed out waiting for the uploader to start streaming")
	}

	// A second completion arrives for the same session while the uploader holds
	// the file open. Only the truncate matters here; CompleteUpload goes on to
	// lose the lock race and return an error, which is precisely the point --
	// the damage is already done by then.
	handler, err := NewHandler(Config{Directory: p.scanDir, OpenFile: os.OpenFile})
	require.NoError(t, err)

	upload2, err := handler.CreateUpload(ctx, sid)
	require.NoError(t, err)
	require.NoError(t, handler.ReserveUploadPart(ctx, *upload2, 1))
	part, err := handler.UploadPart(ctx, *upload2, 1, bytes.NewReader([]byte("second completion")))
	require.NoError(t, err)

	completeErr := handler.CompleteUpload(ctx, *upload2, []events.StreamPart{*part})
	t.Logf("concurrent CompleteUpload returned: %v", completeErr)
	t.Logf("recording size: %d bytes before, %d bytes after",
		completeSize, fileSize(t, recordingPath))

	// Let the upload run to completion against whatever is left of the file.
	close(gate.release)

	var uploaderEvent *events.UploadEvent
	var memEvent *events.UploadEvent
	for uploaderEvent == nil {
		select {
		case ev := <-p.memEventsC:
			e := ev
			memEvent = &e
		case ev := <-p.eventsC:
			e := ev
			uploaderEvent = &e
		case <-ctx.Done():
			t.Fatal("timed out waiting for the upload to finish")
		}
	}

	// Failure mode 1: the reader hits the truncated file and the upload errors.
	// The recording is not delivered, and on the next scan it is moved to
	// CorruptedDir -- for which there is no metric.
	require.NoError(t, uploaderEvent.Error,
		"RECORDING LOST: the concurrent CompleteUpload truncated %s out from under the "+
			"uploader, and the upload failed", recordingPath)

	// Failure mode 2: the truncation lands on a part boundary, the reader sees a
	// clean EOF, and the uploader reports success having delivered only part of
	// the session. This is the quieter and worse outcome.
	require.NotNil(t, memEvent, "upload reported success but never reached the audit backend")
	outEvents := p.readEvents(ctx, t, memEvent.UploadID)
	require.Equal(t, inEvents, outEvents,
		"RECORDING TRUNCATED: the uploader reported success but delivered %d of %d events, "+
			"because the concurrent CompleteUpload truncated %s mid-upload",
		len(outEvents), len(inEvents), recordingPath)
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	fi, err := os.Stat(path)
	if os.IsNotExist(err) {
		return 0
	}
	require.NoError(t, err)
	return fi.Size()
}

// gatedStreamer blocks the first RecordEvent of an upload until release is
// closed, holding the uploader in the middle of reading the recording. It
// stands in for a slow or remote audit backend; nothing is injected into
// production code.
type gatedStreamer struct {
	inner   events.Streamer
	reached chan struct{}
	release chan struct{}
	once    sync.Once
}

func newGatedStreamer() *gatedStreamer {
	return &gatedStreamer{
		reached: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (g *gatedStreamer) CreateAuditStream(ctx context.Context, sid session.ID) (apievents.Stream, error) {
	stream, err := g.inner.CreateAuditStream(ctx, sid)
	if err != nil {
		return nil, err
	}
	return &gatedStream{Stream: stream, gate: g}, nil
}

func (g *gatedStreamer) ResumeAuditStream(ctx context.Context, sid session.ID, uploadID string) (apievents.Stream, error) {
	stream, err := g.inner.ResumeAuditStream(ctx, sid, uploadID)
	if err != nil {
		return nil, err
	}
	return &gatedStream{Stream: stream, gate: g}, nil
}

type gatedStream struct {
	apievents.Stream
	gate *gatedStreamer
}

func (s *gatedStream) RecordEvent(ctx context.Context, event apievents.PreparedSessionEvent) error {
	s.gate.once.Do(func() {
		close(s.gate.reached)
		select {
		case <-s.gate.release:
		case <-ctx.Done():
		}
	})
	return s.Stream.RecordEvent(ctx, event)
}
