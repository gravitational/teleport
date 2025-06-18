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
	"bytes"
	"context"
	"crypto/rand"
	"errors"
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

// TestUploadOK tests async file uploads scenarios
func TestUploadOK(t *testing.T) {
	p := newUploaderPack(t, nil)
	defer p.Close(t)

	// wait until uploader blocks on the clock
	p.clock.BlockUntil(1)

	fileStreamer, err := NewStreamer(p.scanDir, nil)
	require.NoError(t, err)

	inEvents := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 1024})
	sid := inEvents[0].(events.SessionMetadataGetter).GetSessionID()

	emitStream(p.ctx, t, fileStreamer, inEvents)

	// initiate the scan by advancing clock past
	// block period
	p.clock.Advance(p.scanPeriod + time.Second)

	var event events.UploadEvent
	select {
	case event = <-p.memEventsC:
		require.Equal(t, event.SessionID, sid)
		require.NoError(t, event.Error)
	case <-p.ctx.Done():
		t.Fatalf("Timeout waiting for async upload, try `go test -v` to get more logs for details")
	}

	// read the upload and make sure the data is equal
	outEvents := readStream(p.ctx, t, event.UploadID, p.memUploader)
	require.Equal(t, inEvents, outEvents)
}

// TestUploadParallel verifies several parallel uploads that have to wait
// for semaphore
func TestUploadParallel(t *testing.T) {
	p := newUploaderPack(t, nil)
	defer p.Close(t)

	// wait until uploader blocks on the clock
	p.clock.BlockUntil(1)

	sessions := make(map[string][]apievents.AuditEvent)

	for i := 0; i < 5; i++ {
		fileStreamer, err := NewStreamer(p.scanDir, nil)
		require.NoError(t, err)

		sessionEvents := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 1024})
		sid := sessionEvents[0].(events.SessionMetadataGetter).GetSessionID()

		emitStream(p.ctx, t, fileStreamer, sessionEvents)
		sessions[sid] = sessionEvents
	}

	// initiate the scan by advancing the clock past
	// block period
	p.clock.Advance(p.scanPeriod + time.Second)

	for range sessions {
		var event events.UploadEvent
		var sessionEvents []apievents.AuditEvent
		var found bool
		select {
		case event = <-p.memEventsC:
			require.NoError(t, event.Error)
			sessionEvents, found = sessions[event.SessionID]
			require.True(t, found, "session %q is not expected, possible duplicate event", event.SessionID)
		case <-p.ctx.Done():
			t.Fatalf("Timeout waiting for async upload, try `go test -v` to get more logs for details")
		}

		// read the upload and make sure the data is equal
		outEvents := readStream(p.ctx, t, event.UploadID, p.memUploader)

		require.Equal(t, sessionEvents, outEvents)

		delete(sessions, event.SessionID)
	}
}

// TestMovesCorruptedUploads verifies that the uploader moves corrupted uploads
// out of the scan directory.
func TestMovesCorruptedUploads(t *testing.T) {
	scanDir := t.TempDir()
	corruptedDir := t.TempDir()

	uploader, err := NewUploader(UploaderConfig{
		Streamer:     events.NewDiscardStreamer(),
		ScanDir:      scanDir,
		CorruptedDir: corruptedDir,
	})
	require.NoError(t, err)

	sessionID := session.NewID()
	uploadPath := filepath.Join(scanDir, sessionID.String()+".tar")
	errorPath := filepath.Join(scanDir, sessionID.String()+".error")
	badFilePath := filepath.Join(scanDir, "not-a-uuid.tar")

	// create a "corrupted" upload and error file in the scan dir
	b := make([]byte, 4096)
	rand.Read(b)
	require.NoError(t, os.WriteFile(uploadPath, b, 0o600))
	require.NoError(t, uploader.writeSessionError(sessionID, errors.New("this is a corrupted upload")))

	// create a file with an invalid name (not a session ID)
	badFile, err := os.Create(badFilePath)
	require.NoError(t, err)
	require.NoError(t, badFile.Close())

	stats, err := uploader.Scan(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, stats.Scanned)
	require.Equal(t, 2, stats.Corrupted)
	require.Equal(t, 0, stats.Started)

	require.NoFileExists(t, uploadPath)
	require.NoFileExists(t, errorPath)

	require.FileExists(t, filepath.Join(corruptedDir, filepath.Base(uploadPath)))
	require.FileExists(t, filepath.Join(corruptedDir, filepath.Base(errorPath)))

	// run a second scan to verify that:
	// 1. the corrupted file is no longer processed
	// 2. the file with the bad name was still flagged as corrupted
	stats, err = uploader.Scan(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, stats.Scanned)
	require.Equal(t, 1, stats.Corrupted)
	require.Equal(t, 0, stats.Started)
}

type resumeTestCase struct {
	name    string
	newTest func(streamer events.Streamer) resumeTestTuple
	// retries is how many times the uploader will retry the upload
	// after the first upload attempt fails
	retries int
	// onRetry is called on retry attempt
	onRetry func(t *testing.T, attempt int, uploader *Uploader)
}

type resumeTestTuple struct {
	streamer *events.CallbackStreamer
	verify   func(t *testing.T, tc resumeTestCase)
}

// TestUploadResume verifies successful upload run after the stream has been interrupted
func TestUploadResume(t *testing.T) {
	testCases := []resumeTestCase{
		{
			name:    "stream terminates in the middle of submission",
			retries: 1,
			newTest: func(streamer events.Streamer) resumeTestTuple {
				var streamResumed, terminateConnection atomic.Uint64
				terminateConnection.Store(1)

				callbackStreamer, err := events.NewCallbackStreamer(events.CallbackStreamerConfig{
					Inner: streamer,
					OnRecordEvent: func(ctx context.Context, sid session.ID, pe apievents.PreparedSessionEvent) error {
						event := pe.GetAuditEvent()
						if event.GetIndex() > 600 && terminateConnection.CompareAndSwap(1, 0) == true {
							t.Logf("Terminating connection at event %v", event.GetIndex())
							return trace.ConnectionProblem(nil, "connection terminated")
						}
						return nil
					},
					OnResumeAuditStream: func(ctx context.Context, sid session.ID, uploadID string, streamer events.Streamer) (apievents.Stream, error) {
						stream, err := streamer.ResumeAuditStream(ctx, sid, uploadID)
						require.NoError(t, err)
						streamResumed.Add(1)
						return stream, nil
					},
				})
				require.NoError(t, err)
				return resumeTestTuple{
					streamer: callbackStreamer,
					verify: func(t *testing.T, tc resumeTestCase) {
						require.Equal(t, 1, int(streamResumed.Load()), tc.name)
					},
				}
			},
		},
		{
			name:    "stream terminates multiple times at different stages of submission",
			retries: 10,
			newTest: func(streamer events.Streamer) resumeTestTuple {
				var streamResumed, terminateConnection atomic.Uint64

				callbackStreamer, err := events.NewCallbackStreamer(events.CallbackStreamerConfig{
					Inner: streamer,
					OnRecordEvent: func(ctx context.Context, sid session.ID, pe apievents.PreparedSessionEvent) error {
						event := pe.GetAuditEvent()
						if event.GetIndex() > 600 && terminateConnection.Add(1) <= 10 {
							t.Logf("Terminating connection #%v at event %v", terminateConnection.Load(), event.GetIndex())
							return trace.ConnectionProblem(nil, "connection terminated")
						}
						return nil
					},
					OnResumeAuditStream: func(ctx context.Context, sid session.ID, uploadID string, streamer events.Streamer) (apievents.Stream, error) {
						stream, err := streamer.ResumeAuditStream(ctx, sid, uploadID)
						require.NoError(t, err)
						streamResumed.Add(1)
						return stream, nil
					},
				})
				require.NoError(t, err)
				return resumeTestTuple{
					streamer: callbackStreamer,
					verify: func(t *testing.T, tc resumeTestCase) {
						require.Equal(t, 10, int(streamResumed.Load()), tc.name)
					},
				}
			},
		},
		{
			name:    "stream resumes if upload is not found",
			retries: 1,
			newTest: func(streamer events.Streamer) resumeTestTuple {
				var streamCreated, terminateConnection atomic.Uint64
				terminateConnection.Store(1)

				callbackStreamer, err := events.NewCallbackStreamer(events.CallbackStreamerConfig{
					Inner: streamer,
					OnRecordEvent: func(ctx context.Context, sid session.ID, pe apievents.PreparedSessionEvent) error {
						event := pe.GetAuditEvent()
						if event.GetIndex() > 600 && terminateConnection.CompareAndSwap(1, 0) == true {
							t.Logf("Terminating connection at event %v", event.GetIndex())
							return trace.ConnectionProblem(nil, "connection terminated")
						}
						return nil
					},
					OnCreateAuditStream: func(ctx context.Context, sid session.ID, streamer events.Streamer) (apievents.Stream, error) {
						stream, err := streamer.CreateAuditStream(ctx, sid)
						require.NoError(t, err)
						streamCreated.Add(1)
						return stream, nil
					},
					OnResumeAuditStream: func(ctx context.Context, sid session.ID, uploadID string, streamer events.Streamer) (apievents.Stream, error) {
						return nil, trace.NotFound("stream not found")
					},
				})
				require.NoError(t, err)
				return resumeTestTuple{
					streamer: callbackStreamer,
					verify: func(t *testing.T, tc resumeTestCase) {
						require.Equal(t, 2, int(streamCreated.Load()), tc.name)
					},
				}
			},
		},
		{
			name:    "stream created when checkpoint is lost after failure",
			retries: 1,
			onRetry: func(t *testing.T, attempt int, uploader *Uploader) {
				files, err := os.ReadDir(uploader.cfg.ScanDir)
				require.NoError(t, err)
				checkpointsDeleted := 0
				for i := range files {
					fi := files[i]
					if fi.IsDir() {
						continue
					}
					if filepath.Ext(fi.Name()) == checkpointExt {
						err := os.Remove(filepath.Join(uploader.cfg.ScanDir, fi.Name()))
						require.NoError(t, err)
						t.Logf("Deleted checkpoint file: %v.", fi.Name())
						checkpointsDeleted++
					}
				}
				require.Equal(t, 1, checkpointsDeleted, "expected to delete checkpoint file")
			},
			newTest: func(streamer events.Streamer) resumeTestTuple {
				var streamCreated, terminateConnection, streamResumed atomic.Uint64
				terminateConnection.Store(1)

				callbackStreamer, err := events.NewCallbackStreamer(events.CallbackStreamerConfig{
					Inner: streamer,
					OnRecordEvent: func(ctx context.Context, sid session.ID, pe apievents.PreparedSessionEvent) error {
						event := pe.GetAuditEvent()
						if event.GetIndex() > 600 && terminateConnection.CompareAndSwap(1, 0) == true {
							t.Logf("Terminating connection at event %v", event.GetIndex())
							return trace.ConnectionProblem(nil, "connection terminated")
						}
						return nil
					},
					OnCreateAuditStream: func(ctx context.Context, sid session.ID, streamer events.Streamer) (apievents.Stream, error) {
						stream, err := streamer.CreateAuditStream(ctx, sid)
						require.NoError(t, err)
						streamCreated.Add(1)
						return stream, nil
					},
					OnResumeAuditStream: func(ctx context.Context, sid session.ID, uploadID string, streamer events.Streamer) (apievents.Stream, error) {
						stream, err := streamer.ResumeAuditStream(ctx, sid, uploadID)
						require.NoError(t, err)
						streamResumed.Add(1)
						return stream, nil
					},
				})
				require.NoError(t, err)
				return resumeTestTuple{
					streamer: callbackStreamer,
					verify: func(t *testing.T, tc resumeTestCase) {
						require.Equal(t, 2, int(streamCreated.Load()), tc.name)
						require.Equal(t, 0, int(streamResumed.Load()), tc.name)
					},
				}
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runResume(t, tc)
		})
	}
}

// TestUploadBackoff introduces upload failure
// and makes sure that the uploader starts backing off.
func TestUploadBackoff(t *testing.T) {
	var terminateConnectionAt atomic.Int64
	terminateConnectionAt.Store(700)

	p := newUploaderPack(t, func(streamer events.Streamer) (events.Streamer, error) {
		return events.NewCallbackStreamer(events.CallbackStreamerConfig{
			Inner: streamer,
			OnRecordEvent: func(ctx context.Context, sid session.ID, pe apievents.PreparedSessionEvent) error {
				event := pe.GetAuditEvent()
				terminateAt := terminateConnectionAt.Load()
				if terminateAt > 0 && event.GetIndex() >= terminateAt {
					t.Logf("Terminating connection at event %v", event.GetIndex())
					return trace.ConnectionProblem(nil, "connection terminated at event index %v", terminateAt)
				}
				return nil
			},
		})
	})
	defer p.Close(t)

	// wait until uploader blocks on the clock before creating the stream
	p.clock.BlockUntil(1)

	fileStreamer, err := NewStreamer(p.scanDir, nil)
	require.NoError(t, err)

	inEvents := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 4096})
	sid := inEvents[0].(events.SessionMetadataGetter).GetSessionID()

	stream, err := fileStreamer.CreateAuditStream(p.ctx, session.ID(sid))
	require.NoError(t, err)

	for _, event := range inEvents {
		err := stream.RecordEvent(p.ctx, eventstest.PrepareEvent(event))
		require.NoError(t, err)
	}
	err = stream.Complete(p.ctx)
	require.NoError(t, err)

	// initiate the scan by advancing clock past
	// block period
	p.clock.Advance(p.scanPeriod + time.Second)

	// initiate several scan attempts and increase the scan period
	attempts := 10
	var prev time.Time
	var diffs []time.Duration
	for i := 0; i < attempts; i++ {
		// wait for the upload event
		var event events.UploadEvent
		select {
		case event = <-p.eventsC:
			require.Error(t, event.Error)
			if prev.IsZero() {
				prev = event.Created
			} else {
				diffs = append(diffs, event.Created.Sub(prev))
				prev = event.Created
			}
		case <-p.ctx.Done():
			t.Fatalf("Timeout waiting for async upload %v, try `go test -v` to get more logs for details", i)
		}

		// Block until Scan has been called two times,
		// first time after doing the scan, and second
		// on receiving the event to <- eventsCh
		p.clock.BlockUntil(2)
		p.clock.Advance(p.scanPeriod*time.Duration(i+2) + time.Second)
	}

	// Make sure that durations between retries are increasing
	for i, diff := range diffs {
		if i > 0 {
			require.Greater(t, diff, diffs[i-1], "Expected next retry to take longer, got %v vs %v", diffs[i-1], diff)
		}
	}

	// Fix the streamer, make sure the upload succeeds
	terminateConnectionAt.Store(0)
	p.clock.BlockUntil(2)
	p.clock.Advance(time.Hour)
	select {
	case event := <-p.eventsC:
		require.NoError(t, event.Error)
	case <-p.ctx.Done():
		t.Fatalf("Timeout waiting for async upload, try `go test -v` to get more logs for details")
	}
}

// TestUploadBadSession creates a corrupted session file
// and makes sure the uploader marks it as faulty
func TestUploadBadSession(t *testing.T) {
	ctx := context.Background()

	p := newUploaderPack(t, nil)
	defer p.Close(t)

	// wait until uploader blocks on the clock
	p.clock.BlockUntil(1)

	sessionID := session.NewID()
	fileName := filepath.Join(p.scanDir, string(sessionID)+tarExt)

	err := os.WriteFile(fileName, []byte("this session is corrupted"), 0o600)
	require.NoError(t, err)

	// initiate the scan by advancing clock past
	// block period
	p.clock.Advance(p.scanPeriod + time.Second)

	// wait for the upload event, make sure
	// the error is the problem error
	var event events.UploadEvent
	select {
	case event = <-p.eventsC:
		require.Error(t, event.Error)
		require.True(t, isSessionError(event.Error))
	case <-p.ctx.Done():
		t.Fatalf("Timeout waiting for async upload, try `go test -v` to get more logs for details")
	}

	stats, err := p.uploader.Scan(ctx)
	require.NoError(t, err)
	// Bad records have been scanned, but uploads have not started
	require.Equal(t, 1, stats.Scanned)
	require.Equal(t, 0, stats.Started)
}

// uploaderPack reduces boilerplate required
// to create a test
type uploaderPack struct {
	scanPeriod       time.Duration
	initialScanDelay time.Duration
	clock            *clockwork.FakeClock
	eventsC          chan events.UploadEvent
	memEventsC       chan events.UploadEvent
	memUploader      *eventstest.MemoryUploader
	streamer         events.Streamer
	scanDir          string
	uploader         *Uploader
	ctx              context.Context
	cancel           context.CancelFunc
}

func (u *uploaderPack) Close(t *testing.T) {
	u.cancel()
}

func newUploaderPack(t *testing.T, wrapStreamer wrapStreamerFn) uploaderPack {
	scanDir := t.TempDir()
	corruptedDir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	pack := uploaderPack{
		clock:            clockwork.NewFakeClock(),
		eventsC:          make(chan events.UploadEvent, 100),
		memEventsC:       make(chan events.UploadEvent, 100),
		ctx:              ctx,
		cancel:           cancel,
		scanDir:          scanDir,
		scanPeriod:       10 * time.Second,
		initialScanDelay: 10 * time.Millisecond,
	}
	pack.memUploader = eventstest.NewMemoryUploader(pack.memEventsC)

	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader:       pack.memUploader,
		MinUploadBytes: 1024,
	})
	require.NoError(t, err)
	pack.streamer = streamer
	if wrapStreamer != nil {
		pack.streamer, err = wrapStreamer(pack.streamer)
		require.NoError(t, err)
	}

	uploader, err := NewUploader(UploaderConfig{
		ScanDir:          pack.scanDir,
		CorruptedDir:     corruptedDir,
		InitialScanDelay: pack.initialScanDelay,
		ScanPeriod:       pack.scanPeriod,
		Streamer:         pack.streamer,
		Clock:            pack.clock,
		EventsC:          pack.eventsC,
	})
	require.NoError(t, err)
	pack.uploader = uploader
	go pack.uploader.Serve(pack.ctx)
	return pack
}

type wrapStreamerFn func(streamer events.Streamer) (events.Streamer, error)

// runResume runs resume scenario based on the test case specification
func runResume(t *testing.T, testCase resumeTestCase) {
	t.Logf("Running test %q.", testCase.name)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clock := clockwork.NewFakeClock()
	eventsC := make(chan events.UploadEvent, 100)
	memUploader := eventstest.NewMemoryUploader(eventsC)
	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader:       memUploader,
		MinUploadBytes: 1024,
	})
	require.NoError(t, err)

	test := testCase.newTest(streamer)

	scanDir := t.TempDir()
	corruptedDir := t.TempDir()

	scanPeriod := 10 * time.Second
	uploader, err := NewUploader(UploaderConfig{
		EventsC:          eventsC,
		ScanDir:          scanDir,
		CorruptedDir:     corruptedDir,
		InitialScanDelay: 10 * time.Millisecond,
		ScanPeriod:       scanPeriod,
		Streamer:         test.streamer,
		Clock:            clock,
	})
	require.NoError(t, err)
	go uploader.Serve(ctx)
	// wait until uploader blocks on the clock
	clock.BlockUntil(1)

	defer uploader.Close()

	fileStreamer, err := NewStreamer(scanDir, nil)
	require.NoError(t, err)

	inEvents := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 1024})
	sid := inEvents[0].(events.SessionMetadataGetter).GetSessionID()

	emitStream(ctx, t, fileStreamer, inEvents)

	// initiate the scan by advancing clock past
	// block period
	clock.Advance(scanPeriod + time.Second)

	// wait for the upload failure
	var event events.UploadEvent
	select {
	case event = <-eventsC:
		require.Equal(t, event.SessionID, sid)
		require.IsType(t, trace.ConnectionProblem(nil, "connection problem"), event.Error)
	case <-ctx.Done():
		t.Fatalf("Timeout waiting for async upload, try `go test -v` to get more logs for details")
	}

	for i := 0; i < testCase.retries; i++ {
		if testCase.onRetry != nil {
			testCase.onRetry(t, i, uploader)
		}
		// Block until Scan has been called two times,
		// first time after doing the scan, and second
		// on receiving the event to <- eventsCh
		clock.BlockUntil(2)
		clock.Advance(scanPeriod*time.Duration(i+2) + time.Second)

		// wait for upload success
		select {
		case event = <-eventsC:
			require.Equal(t, event.SessionID, sid)
			if i == testCase.retries-1 {
				require.NoError(t, event.Error)
			} else {
				require.IsType(t, trace.ConnectionProblem(nil, "connection problem"), event.Error)
			}
		case <-ctx.Done():
			t.Fatalf("Timeout waiting for async upload, try `go test -v` to get more logs for details")
		}
	}

	// read the upload and make sure the data is equal
	outEvents := readStream(ctx, t, event.UploadID, memUploader)

	require.Equal(t, inEvents, outEvents)

	// perform additional checks as defined by test case
	test.verify(t, testCase)
}

// emitStream creates and sends the session stream
func emitStream(ctx context.Context, t *testing.T, streamer events.Streamer, inEvents []apievents.AuditEvent) {
	sid := inEvents[0].(events.SessionMetadataGetter).GetSessionID()

	stream, err := streamer.CreateAuditStream(ctx, session.ID(sid))
	require.NoError(t, err)
	for _, event := range inEvents {
		err := stream.RecordEvent(ctx, eventstest.PrepareEvent(event))
		require.NoError(t, err)
	}
	err = stream.Complete(ctx)
	require.NoError(t, err)
}

// readStream reads and decodes the audit stream from uploadID
func readStream(ctx context.Context, t *testing.T, uploadID string, uploader *eventstest.MemoryUploader) []apievents.AuditEvent {
	t.Helper()

	parts, err := uploader.GetParts(uploadID)
	require.NoError(t, err)

	var outEvents []apievents.AuditEvent
	var reader *events.ProtoReader
	for i, part := range parts {
		if i == 0 {
			reader = events.NewProtoReader(bytes.NewReader(part), nil)
		} else {
			err := reader.Reset(bytes.NewReader(part))
			require.NoError(t, err)
		}
		out, err := reader.ReadAll(ctx)
		require.NoError(t, err, "part crash %#v", part)

		outEvents = append(outEvents, out...)
	}
	return outEvents
}
