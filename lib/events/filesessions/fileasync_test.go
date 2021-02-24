/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

*/

package filesessions

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"

	"github.com/gravitational/trace"
)

// TestUploadOK tests async file uploads scenarios
func TestUploadOK(t *testing.T) {
	p := newUploaderPack(t, nil)
	defer p.Close(t)

	// wait until uploader blocks on the clock
	p.clock.BlockUntil(1)

	fileStreamer, err := NewStreamer(p.scanDir)
	require.Nil(t, err)

	inEvents := events.GenerateTestSession(events.SessionParams{PrintEvents: 1024})
	sid := inEvents[0].(events.SessionMetadataGetter).GetSessionID()

	emitStream(p.ctx, t, fileStreamer, inEvents)

	// initiate the scan by advancing clock past
	// block period
	p.clock.Advance(p.scanPeriod + time.Second)

	var event events.UploadEvent
	select {
	case event = <-p.memEventsC:
		require.Equal(t, event.SessionID, sid)
		require.Nil(t, event.Error)
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

	sessions := make(map[string][]events.AuditEvent)

	for i := 0; i < 5; i++ {
		fileStreamer, err := NewStreamer(p.scanDir)
		require.Nil(t, err)

		sessionEvents := events.GenerateTestSession(events.SessionParams{PrintEvents: 1024})
		sid := sessionEvents[0].(events.SessionMetadataGetter).GetSessionID()

		emitStream(p.ctx, t, fileStreamer, sessionEvents)
		sessions[sid] = sessionEvents
	}

	// initiate the scan by advancing the clock past
	// block period
	p.clock.Advance(p.scanPeriod + time.Second)

	for range sessions {
		var event events.UploadEvent
		var sessionEvents []events.AuditEvent
		var found bool
		select {
		case event = <-p.memEventsC:
			require.Nil(t, event.Error)
			sessionEvents, found = sessions[event.SessionID]
			require.Equal(t, found, true,
				"session %q is not expected, possible duplicate event", event.SessionID)
		case <-p.ctx.Done():
			t.Fatalf("Timeout waiting for async upload, try `go test -v` to get more logs for details")
		}

		// read the upload and make sure the data is equal
		outEvents := readStream(p.ctx, t, event.UploadID, p.memUploader)

		require.Equal(t, sessionEvents, outEvents)

		delete(sessions, event.SessionID)
	}
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
				streamResumed := atomic.NewUint64(0)
				terminateConnection := atomic.NewUint64(1)

				callbackStreamer, err := events.NewCallbackStreamer(events.CallbackStreamerConfig{
					Inner: streamer,
					OnEmitAuditEvent: func(ctx context.Context, sid session.ID, event events.AuditEvent) error {
						if event.GetIndex() > 600 && terminateConnection.CAS(1, 0) == true {
							log.Debugf("Terminating connection at event %v", event.GetIndex())
							return trace.ConnectionProblem(nil, "connection terminated")
						}
						return nil
					},
					OnResumeAuditStream: func(ctx context.Context, sid session.ID, uploadID string, streamer events.Streamer) (events.Stream, error) {
						stream, err := streamer.ResumeAuditStream(ctx, sid, uploadID)
						require.Nil(t, err)
						streamResumed.Inc()
						return stream, nil
					},
				})
				require.Nil(t, err)
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
				streamResumed := atomic.NewUint64(0)
				terminateConnection := atomic.NewUint64(0)

				callbackStreamer, err := events.NewCallbackStreamer(events.CallbackStreamerConfig{
					Inner: streamer,
					OnEmitAuditEvent: func(ctx context.Context, sid session.ID, event events.AuditEvent) error {
						if event.GetIndex() > 600 && terminateConnection.Inc() <= 10 {
							log.Debugf("Terminating connection #%v at event %v", terminateConnection.Load(), event.GetIndex())
							return trace.ConnectionProblem(nil, "connection terminated")
						}
						return nil
					},
					OnResumeAuditStream: func(ctx context.Context, sid session.ID, uploadID string, streamer events.Streamer) (events.Stream, error) {
						stream, err := streamer.ResumeAuditStream(ctx, sid, uploadID)
						require.Nil(t, err)
						streamResumed.Inc()
						return stream, nil
					},
				})
				require.Nil(t, err)
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
				streamCreated := atomic.NewUint64(0)
				terminateConnection := atomic.NewUint64(1)

				callbackStreamer, err := events.NewCallbackStreamer(events.CallbackStreamerConfig{
					Inner: streamer,
					OnEmitAuditEvent: func(ctx context.Context, sid session.ID, event events.AuditEvent) error {
						if event.GetIndex() > 600 && terminateConnection.CAS(1, 0) == true {
							log.Debugf("Terminating connection at event %v", event.GetIndex())
							return trace.ConnectionProblem(nil, "connection terminated")
						}
						return nil
					},
					OnCreateAuditStream: func(ctx context.Context, sid session.ID, streamer events.Streamer) (events.Stream, error) {
						stream, err := streamer.CreateAuditStream(ctx, sid)
						require.Nil(t, err)
						streamCreated.Inc()
						return stream, nil
					},
					OnResumeAuditStream: func(ctx context.Context, sid session.ID, uploadID string, streamer events.Streamer) (events.Stream, error) {
						return nil, trace.NotFound("stream not found")
					},
				})
				require.Nil(t, err)
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
				files, err := ioutil.ReadDir(uploader.cfg.ScanDir)
				require.Nil(t, err)
				checkpointsDeleted := 0
				for i := range files {
					fi := files[i]
					if fi.IsDir() {
						continue
					}
					if filepath.Ext(fi.Name()) == checkpointExt {
						err := os.Remove(filepath.Join(uploader.cfg.ScanDir, fi.Name()))
						require.Nil(t, err)
						log.Debugf("Deleted checkpoint file: %v.", fi.Name())
						checkpointsDeleted++
					}
				}
				require.Equal(t, 1, checkpointsDeleted, "expected to delete checkpoint file")
			},
			newTest: func(streamer events.Streamer) resumeTestTuple {
				streamCreated := atomic.NewUint64(0)
				terminateConnection := atomic.NewUint64(1)
				streamResumed := atomic.NewUint64(0)

				callbackStreamer, err := events.NewCallbackStreamer(events.CallbackStreamerConfig{
					Inner: streamer,
					OnEmitAuditEvent: func(ctx context.Context, sid session.ID, event events.AuditEvent) error {
						if event.GetIndex() > 600 && terminateConnection.CAS(1, 0) == true {
							log.Debugf("Terminating connection at event %v", event.GetIndex())
							return trace.ConnectionProblem(nil, "connection terminated")
						}
						return nil
					},
					OnCreateAuditStream: func(ctx context.Context, sid session.ID, streamer events.Streamer) (events.Stream, error) {
						stream, err := streamer.CreateAuditStream(ctx, sid)
						require.Nil(t, err)
						streamCreated.Inc()
						return stream, nil
					},
					OnResumeAuditStream: func(ctx context.Context, sid session.ID, uploadID string, streamer events.Streamer) (events.Stream, error) {
						stream, err := streamer.ResumeAuditStream(ctx, sid, uploadID)
						require.Nil(t, err)
						streamResumed.Inc()
						return stream, nil
					},
				})
				require.Nil(t, err)
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
	terminateConnectionAt := atomic.NewInt64(700)

	p := newUploaderPack(t, func(streamer events.Streamer) (events.Streamer, error) {
		return events.NewCallbackStreamer(events.CallbackStreamerConfig{
			Inner: streamer,
			OnEmitAuditEvent: func(ctx context.Context, sid session.ID, event events.AuditEvent) error {
				terminateAt := terminateConnectionAt.Load()
				if terminateAt > 0 && event.GetIndex() >= terminateAt {
					log.Debugf("Terminating connection at event %v", event.GetIndex())
					return trace.ConnectionProblem(nil, "connection terminated at event index %v", terminateAt)
				}
				return nil
			},
		})
	})
	defer p.Close(t)

	// wait until uploader blocks on the clock before creating the stream
	p.clock.BlockUntil(1)

	fileStreamer, err := NewStreamer(p.scanDir)
	require.NoError(t, err)

	inEvents := events.GenerateTestSession(events.SessionParams{PrintEvents: 4096})
	sid := inEvents[0].(events.SessionMetadataGetter).GetSessionID()

	stream, err := fileStreamer.CreateAuditStream(p.ctx, session.ID(sid))
	require.NoError(t, err)

	for _, event := range inEvents {
		err := stream.EmitAuditEvent(p.ctx, event)
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
			require.True(t, diff > diffs[i-1], "Expected next retry to take longer, got %v vs %v", diffs[i-1], diff)
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
	p := newUploaderPack(t, nil)
	defer p.Close(t)

	// wait until uploader blocks on the clock
	p.clock.BlockUntil(1)

	sessionID := session.NewID()
	fileName := filepath.Join(p.scanDir, string(sessionID)+tarExt)

	err := ioutil.WriteFile(fileName, []byte("this session is corrupted"), 0600)
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

	stats, err := p.uploader.Scan()
	require.NoError(t, err)
	// Bad records have been scanned, but uploads have not started
	require.Equal(t, 1, stats.Scanned)
	require.Equal(t, 0, stats.Started)
}

// uploaderPack reduces boilerplate required
// to create a test
type uploaderPack struct {
	scanPeriod  time.Duration
	clock       clockwork.FakeClock
	eventsC     chan events.UploadEvent
	memEventsC  chan events.UploadEvent
	memUploader *events.MemoryUploader
	streamer    events.Streamer
	scanDir     string
	uploader    *Uploader
	ctx         context.Context
	cancel      context.CancelFunc
}

func (u *uploaderPack) Close(t *testing.T) {
	u.cancel()

	err := u.uploader.Close()
	require.NoError(t, err)

	if u.scanDir != "" {
		err := os.RemoveAll(u.scanDir)
		require.NoError(t, err)
	}
}

func newUploaderPack(t *testing.T, wrapStreamer wrapStreamerFn) uploaderPack {
	scanDir, err := ioutil.TempDir("", "teleport-streams")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	pack := uploaderPack{
		clock:      clockwork.NewFakeClock(),
		eventsC:    make(chan events.UploadEvent, 100),
		memEventsC: make(chan events.UploadEvent, 100),
		ctx:        ctx,
		cancel:     cancel,
		scanDir:    scanDir,
		scanPeriod: 10 * time.Second,
	}
	pack.memUploader = events.NewMemoryUploader(pack.memEventsC)

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
		Context:    pack.ctx,
		ScanDir:    pack.scanDir,
		ScanPeriod: pack.scanPeriod,
		Streamer:   pack.streamer,
		Clock:      pack.clock,
		EventsC:    pack.eventsC,
		AuditLog:   &events.DiscardAuditLog{},
	})
	require.NoError(t, err)
	pack.uploader = uploader
	go pack.uploader.Serve()
	return pack
}

type wrapStreamerFn func(streamer events.Streamer) (events.Streamer, error)

// runResume runs resume scenario based on the test case specification
func runResume(t *testing.T, testCase resumeTestCase) {
	log.Debugf("Running test %q.", testCase.name)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clock := clockwork.NewFakeClock()
	eventsC := make(chan events.UploadEvent, 100)
	memUploader := events.NewMemoryUploader(eventsC)
	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader:       memUploader,
		MinUploadBytes: 1024,
	})
	require.Nil(t, err)

	test := testCase.newTest(streamer)

	scanDir, err := ioutil.TempDir("", "teleport-streams")
	require.Nil(t, err)
	defer os.RemoveAll(scanDir)

	scanPeriod := 10 * time.Second
	uploader, err := NewUploader(UploaderConfig{
		EventsC:    eventsC,
		Context:    ctx,
		ScanDir:    scanDir,
		ScanPeriod: scanPeriod,
		Streamer:   test.streamer,
		Clock:      clock,
		AuditLog:   &events.DiscardAuditLog{},
	})
	require.Nil(t, err)
	go uploader.Serve()
	// wait until uploader blocks on the clock
	clock.BlockUntil(1)

	defer uploader.Close()

	fileStreamer, err := NewStreamer(scanDir)
	require.Nil(t, err)

	inEvents := events.GenerateTestSession(events.SessionParams{PrintEvents: 1024})
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
				require.Nil(t, event.Error)
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
func emitStream(ctx context.Context, t *testing.T, streamer events.Streamer, inEvents []events.AuditEvent) {
	sid := inEvents[0].(events.SessionMetadataGetter).GetSessionID()

	stream, err := streamer.CreateAuditStream(ctx, session.ID(sid))
	require.Nil(t, err)
	for _, event := range inEvents {
		err := stream.EmitAuditEvent(ctx, event)
		require.Nil(t, err)
	}
	err = stream.Complete(ctx)
	require.Nil(t, err)
}

// readStream reads and decodes the audit stream from uploadID
func readStream(ctx context.Context, t *testing.T, uploadID string, uploader *events.MemoryUploader) []events.AuditEvent {
	parts, err := uploader.GetParts(uploadID)
	require.Nil(t, err)

	var outEvents []events.AuditEvent
	var reader *events.ProtoReader
	for i, part := range parts {
		if i == 0 {
			reader = events.NewProtoReader(bytes.NewReader(part))
		} else {
			err := reader.Reset(bytes.NewReader(part))
			require.Nil(t, err)
		}
		out, err := reader.ReadAll(ctx)
		require.Nil(t, err, "part crash %#v", part)
		outEvents = append(outEvents, out...)
	}
	return outEvents
}
