//go:build !race

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

package filesessions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/session"
)

// TestChaosUpload introduces failures in all stages of the async
// upload process and verifies that the system is working correctly.
//
// Data race detector slows down the test significantly (10x+),
// that is why the test is skipped when tests are running with
// `go test -race` flag or `go test -short` flag
func TestChaosUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode.")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventsC := make(chan events.UploadEvent, 100)
	memUploader := eventstest.NewMemoryUploader(eventsC)
	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader:       memUploader,
		MinUploadBytes: 1024,
	})
	require.NoError(t, err)

	scanDir := t.TempDir()
	corruptedDir := t.TempDir()

	var terminateConnection, failCreateAuditStream, failResumeAuditStream atomic.Uint64

	faultyStreamer, err := events.NewCallbackStreamer(events.CallbackStreamerConfig{
		Inner: streamer,
		OnRecordEvent: func(ctx context.Context, sid session.ID, pe apievents.PreparedSessionEvent) error {
			event := pe.GetAuditEvent()
			if event.GetIndex() > 700 && terminateConnection.Add(1) < 5 {
				t.Logf("Terminating connection at event %v", event.GetIndex())
				return trace.ConnectionProblem(nil, "connection terminated")
			}
			return nil
		},
		OnCreateAuditStream: func(ctx context.Context, sid session.ID, streamer events.Streamer) (apievents.Stream, error) {
			if failCreateAuditStream.Add(1) < 5 {
				return nil, trace.ConnectionProblem(nil, "failed to create stream")
			}
			return streamer.CreateAuditStream(ctx, sid)
		},
		OnResumeAuditStream: func(ctx context.Context, sid session.ID, uploadID string, streamer events.Streamer) (apievents.Stream, error) {
			resumed := failResumeAuditStream.Add(1)
			if resumed < 5 {
				// for the first 5 resume attempts, simulate nework failure
				return nil, trace.ConnectionProblem(nil, "failed to resume stream")
			} else if resumed >= 5 && resumed < 8 {
				// for the next several resumes, lose checkpoint file for the stream
				files, err := os.ReadDir(scanDir)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				for _, fi := range files {
					if fi.IsDir() {
						continue
					}
					if fi.Name() == sid.String()+checkpointExt {
						err := os.Remove(filepath.Join(scanDir, fi.Name()))
						require.NoError(t, err)
						t.Logf("Deleted checkpoint file: %v.", fi.Name())
						break
					}
				}
			}
			return streamer.ResumeAuditStream(ctx, sid, uploadID)
		},
	})
	require.NoError(t, err)

	uploader, err := NewUploader(UploaderConfig{
		ScanDir:      scanDir,
		CorruptedDir: corruptedDir,
		ScanPeriod:   3 * time.Second,
		Streamer:     faultyStreamer,
		Clock:        clockwork.NewRealClock(),
	})
	require.NoError(t, err)

	go uploader.Serve(ctx)
	defer uploader.Close()

	fileStreamer, err := NewStreamer(scanDir, nil)
	require.NoError(t, err)

	parallelStreams := 20
	type streamState struct {
		sid    string
		events []apievents.AuditEvent
		err    error
	}
	streamsCh := make(chan streamState, parallelStreams)
	for i := 0; i < parallelStreams; i++ {
		go func() {
			inEvents := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 4096})
			sid := inEvents[0].(events.SessionMetadataGetter).GetSessionID()
			s := streamState{
				sid:    sid,
				events: inEvents,
			}

			stream, err := fileStreamer.CreateAuditStream(ctx, session.ID(sid))
			if err != nil {
				s.err = err
				streamsCh <- s
				return
			}
			for _, event := range inEvents {
				err := stream.RecordEvent(ctx, eventstest.PrepareEvent(event))
				if err != nil {
					s.err = err
					streamsCh <- s
					return
				}
			}
			s.err = stream.Complete(ctx)
			streamsCh <- s
		}()
	}

	// wait for all streams to be completed
	streams := make(map[string]streamState)
	for i := 0; i < parallelStreams; i++ {
		select {
		case status := <-streamsCh:
			require.NoError(t, status.err)
			streams[status.sid] = status
		case <-ctx.Done():
			t.Fatalf("Timeout waiting for parallel stream complete, try `go test -v` to get more logs for details")
		}
	}

	require.Len(t, streams, parallelStreams)

	for i := 0; i < parallelStreams; i++ {
		select {
		case event := <-eventsC:
			require.NoError(t, event.Error)
			require.Contains(t, streams, event.SessionID, "missing stream for session")

			state := streams[event.SessionID]
			outEvents := readStream(ctx, t, event.UploadID, memUploader)
			require.Len(t, state.events, len(outEvents), fmt.Sprintf("event: %v", event))
		case <-ctx.Done():
			t.Fatal("Timeout waiting for async upload, try `go test -v` to get more logs for details")
		}
	}
}
