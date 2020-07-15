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

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/atomic"
)

// TestUploadOK tests async file uploads scenarios
func TestUploadOK(t *testing.T) {
	utils.InitLoggerForTests(testing.Verbose())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clock := clockwork.NewFakeClock()

	eventsC := make(chan events.UploadEvent, 100)
	memUploader := events.NewMemoryUploader(eventsC)
	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader: memUploader,
	})
	assert.Nil(t, err)

	scanDir, err := ioutil.TempDir("", "teleport-streams")
	assert.Nil(t, err)
	defer os.RemoveAll(scanDir)

	scanPeriod := 10 * time.Second
	uploader, err := NewUploader(UploaderConfig{
		Context:    ctx,
		ScanDir:    scanDir,
		ScanPeriod: scanPeriod,
		Streamer:   streamer,
		Clock:      clock,
	})
	assert.Nil(t, err)
	go uploader.Serve()

	// wait until uploader blocks on the clock
	clock.BlockUntil(1)

	defer uploader.Close()

	fileStreamer, err := NewStreamer(scanDir)
	assert.Nil(t, err)

	inEvents := events.GenerateTestSession(events.SessionParams{PrintEvents: 1024})
	sid := inEvents[0].(events.SessionMetadataGetter).GetSessionID()

	emitStream(ctx, t, fileStreamer, inEvents)

	// initiate the scan by advancing clock past
	// block period
	clock.Advance(scanPeriod + time.Second)

	var event events.UploadEvent
	select {
	case event = <-eventsC:
		assert.Equal(t, event.SessionID, sid)
		assert.Nil(t, event.Error)
	case <-ctx.Done():
		t.Fatalf("Timeout waiting for async upload, try `go test -v` to get more logs for details")
	}

	// read the upload and make sure the data is equal
	outEvents := readStream(ctx, t, event.UploadID, memUploader)
	assert.Equal(t, inEvents, outEvents)
}

// TestUploadParallel verifies several parallel uploads that have to wait
// for semaphore
func TestUploadParallel(t *testing.T) {
	utils.InitLoggerForTests(testing.Verbose())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clock := clockwork.NewFakeClock()

	eventsC := make(chan events.UploadEvent, 100)
	memUploader := events.NewMemoryUploader(eventsC)
	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader: memUploader,
	})
	assert.Nil(t, err)

	scanDir, err := ioutil.TempDir("", "teleport-streams")
	assert.Nil(t, err)
	defer os.RemoveAll(scanDir)

	scanPeriod := 10 * time.Second
	uploader, err := NewUploader(UploaderConfig{
		Context:           ctx,
		ScanDir:           scanDir,
		ScanPeriod:        scanPeriod,
		Streamer:          streamer,
		Clock:             clock,
		ConcurrentUploads: 2,
	})
	assert.Nil(t, err)
	go uploader.Serve()
	// wait until uploader blocks on the clock
	clock.BlockUntil(1)

	defer uploader.Close()

	sessions := make(map[string][]events.AuditEvent)

	for i := 0; i < 5; i++ {
		fileStreamer, err := NewStreamer(scanDir)
		assert.Nil(t, err)

		sessionEvents := events.GenerateTestSession(events.SessionParams{PrintEvents: 1024})
		sid := sessionEvents[0].(events.SessionMetadataGetter).GetSessionID()

		emitStream(ctx, t, fileStreamer, sessionEvents)
		sessions[sid] = sessionEvents
	}

	// initiate the scan by advancing the clock past
	// block period
	clock.Advance(scanPeriod + time.Second)

	for range sessions {
		var event events.UploadEvent
		var sessionEvents []events.AuditEvent
		var found bool
		select {
		case event = <-eventsC:
			log.Debugf("Got upload event %v", event)
			assert.Nil(t, event.Error)
			sessionEvents, found = sessions[event.SessionID]
			assert.Equal(t, found, true,
				"session %q is not expected, possible duplicate event", event.SessionID)
		case <-ctx.Done():
			t.Fatalf("Timeout waiting for async upload, try `go test -v` to get more logs for details")
		}

		// read the upload and make sure the data is equal
		outEvents := readStream(ctx, t, event.UploadID, memUploader)

		assert.Equal(t, sessionEvents, outEvents)

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
	utils.InitLoggerForTests(testing.Verbose())
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
						assert.Nil(t, err)
						streamResumed.Inc()
						return stream, nil
					},
				})
				assert.Nil(t, err)
				return resumeTestTuple{
					streamer: callbackStreamer,
					verify: func(t *testing.T, tc resumeTestCase) {
						assert.Equal(t, 1, int(streamResumed.Load()), tc.name)
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
						assert.Nil(t, err)
						streamResumed.Inc()
						return stream, nil
					},
				})
				assert.Nil(t, err)
				return resumeTestTuple{
					streamer: callbackStreamer,
					verify: func(t *testing.T, tc resumeTestCase) {
						assert.Equal(t, 10, int(streamResumed.Load()), tc.name)
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
						assert.Nil(t, err)
						streamCreated.Inc()
						return stream, nil
					},
					OnResumeAuditStream: func(ctx context.Context, sid session.ID, uploadID string, streamer events.Streamer) (events.Stream, error) {
						return nil, trace.NotFound("stream not found")
					},
				})
				assert.Nil(t, err)
				return resumeTestTuple{
					streamer: callbackStreamer,
					verify: func(t *testing.T, tc resumeTestCase) {
						assert.Equal(t, 2, int(streamCreated.Load()), tc.name)
					},
				}
			},
		},
		{
			name:    "stream created when checkpoint is lost after failure",
			retries: 1,
			onRetry: func(t *testing.T, attempt int, uploader *Uploader) {
				files, err := ioutil.ReadDir(uploader.cfg.ScanDir)
				assert.Nil(t, err)
				checkpointsDeleted := 0
				for i := range files {
					fi := files[i]
					if fi.IsDir() {
						continue
					}
					if filepath.Ext(fi.Name()) == checkpointExt {
						err := os.Remove(filepath.Join(uploader.cfg.ScanDir, fi.Name()))
						assert.Nil(t, err)
						log.Debugf("Deleted checkpoint file: %v.", fi.Name())
						checkpointsDeleted++
					}
				}
				assert.Equal(t, 1, checkpointsDeleted, "expected to delete checkpoint file")
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
						assert.Nil(t, err)
						streamCreated.Inc()
						return stream, nil
					},
					OnResumeAuditStream: func(ctx context.Context, sid session.ID, uploadID string, streamer events.Streamer) (events.Stream, error) {
						stream, err := streamer.ResumeAuditStream(ctx, sid, uploadID)
						assert.Nil(t, err)
						streamResumed.Inc()
						return stream, nil
					},
				})
				assert.Nil(t, err)
				return resumeTestTuple{
					streamer: callbackStreamer,
					verify: func(t *testing.T, tc resumeTestCase) {
						assert.Equal(t, 2, int(streamCreated.Load()), tc.name)
						assert.Equal(t, 0, int(streamResumed.Load()), tc.name)
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
	assert.Nil(t, err)

	test := testCase.newTest(streamer)

	scanDir, err := ioutil.TempDir("", "teleport-streams")
	assert.Nil(t, err)
	defer os.RemoveAll(scanDir)

	scanPeriod := 10 * time.Second
	uploader, err := NewUploader(UploaderConfig{
		EventsC:    eventsC,
		Context:    ctx,
		ScanDir:    scanDir,
		ScanPeriod: scanPeriod,
		Streamer:   test.streamer,
		Clock:      clock,
	})
	assert.Nil(t, err)
	go uploader.Serve()
	// wait until uploader blocks on the clock
	clock.BlockUntil(1)

	defer uploader.Close()

	fileStreamer, err := NewStreamer(scanDir)
	assert.Nil(t, err)

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
		assert.Equal(t, event.SessionID, sid)
		assert.IsType(t, trace.ConnectionProblem(nil, "connection problem"), event.Error)
	case <-ctx.Done():
		t.Fatalf("Timeout waiting for async upload, try `go test -v` to get more logs for details")
	}

	for i := 0; i < testCase.retries; i++ {
		if testCase.onRetry != nil {
			testCase.onRetry(t, i, uploader)
		}
		clock.BlockUntil(1)
		clock.Advance(scanPeriod + time.Second)

		// wait for upload success
		select {
		case event = <-eventsC:
			assert.Equal(t, event.SessionID, sid)
			if i == testCase.retries-1 {
				assert.Nil(t, event.Error)
			} else {
				assert.IsType(t, trace.ConnectionProblem(nil, "connection problem"), event.Error)
			}
		case <-ctx.Done():
			t.Fatalf("Timeout waiting for async upload, try `go test -v` to get more logs for details")
		}
	}

	// read the upload and make sure the data is equal
	outEvents := readStream(ctx, t, event.UploadID, memUploader)

	assert.Equal(t, inEvents, outEvents)

	// perform additional checks as defined by test case
	test.verify(t, testCase)
}

// emitStream creates and sends the session stream
func emitStream(ctx context.Context, t *testing.T, streamer events.Streamer, inEvents []events.AuditEvent) {
	sid := inEvents[0].(events.SessionMetadataGetter).GetSessionID()

	stream, err := streamer.CreateAuditStream(ctx, session.ID(sid))
	assert.Nil(t, err)
	for _, event := range inEvents {
		err := stream.EmitAuditEvent(ctx, event)
		assert.Nil(t, err)
	}
	err = stream.Complete(ctx)
	assert.Nil(t, err)
}

// readStream reads and decodes the audit stream from uploadID
func readStream(ctx context.Context, t *testing.T, uploadID string, uploader *events.MemoryUploader) []events.AuditEvent {
	parts, err := uploader.GetParts(uploadID)
	assert.Nil(t, err)

	var outEvents []events.AuditEvent
	var reader *events.ProtoReader
	for i, part := range parts {
		if i == 0 {
			reader = events.NewProtoReader(bytes.NewReader(part))
		} else {
			err := reader.Reset(bytes.NewReader(part))
			assert.Nil(t, err)
		}
		out, err := reader.ReadAll(ctx)
		assert.Nil(t, err, "part crash %#v", part)
		outEvents = append(outEvents, out...)
	}
	return outEvents
}
